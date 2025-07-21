package ai_agent

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"os"
	"sync"

	"github.com/checkmarble/marble-backend/dto/agent_dto"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/checkmarble/marble-backend/usecases/inboxes"
	"github.com/checkmarble/marble-backend/usecases/security"
	"github.com/checkmarble/marble-backend/utils"

	"github.com/google/uuid"
	"github.com/openai/openai-go"
	"github.com/pkg/errors"
)

const HIGH_NB_ROWS_THRESHOLD = 0

type AiAgentUsecaseRepository interface {
	GetCaseById(ctx context.Context, exec repositories.Executor, caseId string) (models.Case, error)
	ListCaseEvents(ctx context.Context, exec repositories.Executor, caseId string) ([]models.CaseEvent, error)
	ListRulesByIterationId(ctx context.Context, exec repositories.Executor, iterationId string) ([]models.Rule, error)
	ListUsers(ctx context.Context, exec repositories.Executor, organizationIDFilter *string) ([]models.User, error)
	DecisionsByCaseId(ctx context.Context, exec repositories.Executor, orgId string, caseId string) (
		[]models.DecisionWithRuleExecutions, error)
	DecisionPivotValuesByCase(ctx context.Context, exec repositories.Executor, caseId string) ([]models.PivotDataWithCount, error)
	GetCasesWithPivotValue(ctx context.Context, exec repositories.Executor,
		orgId, pivotValue string) ([]models.Case, error)
	ListOrganizationTags(ctx context.Context, exec repositories.Executor, organizationId string,
		target models.TagTarget, withCaseCount bool) ([]models.Tag, error)
	GetScenarioIteration(ctx context.Context, exec repositories.Executor, scenarioIterationId string) (models.ScenarioIteration, error)
	ListScreeningsForDecision(ctx context.Context, exec repositories.Executor, decisionId string,
		initialOnly bool) ([]models.ScreeningWithMatches, error)
}

type AiAgentUsecaseIngestedDataReader interface {
	ReadPivotObjectsFromValues(
		ctx context.Context,
		organizationId string,
		values []models.PivotDataWithCount,
	) ([]models.PivotObject, error)
	ReadIngestedClientObjects(
		ctx context.Context,
		orgId string,
		objectType string,
		input models.ClientDataListRequestBody,
		fieldsToRead ...string,
	) (objects []models.ClientObjectDetail, fieldStats []models.FieldStatistics,
		pagination models.ClientDataListPagination, err error)
}

type AiAgentUsecaseDataModelUsecase interface {
	GetDataModel(ctx context.Context, organizationID string, options models.DataModelReadOptions) (models.DataModel, error)
}

type AiAgentUsecase struct {
	enforceSecurity          security.EnforceSecurityCase
	repository               AiAgentUsecaseRepository
	inboxReader              inboxes.InboxReader
	executorFactory          executor_factory.ExecutorFactory
	ingestedDataReader       AiAgentUsecaseIngestedDataReader
	dataModelUsecase         AiAgentUsecaseDataModelUsecase
	caseReviewFileRepository caseReviewFileRepository
	blobRepository           repositories.BlobRepository

	client *openai.Client
	mu     sync.Mutex
}

func NewAiAgentUsecase(
	enforceSecurity security.EnforceSecurityCase,
	repository AiAgentUsecaseRepository,
	inboxReader inboxes.InboxReader,
	executorFactory executor_factory.ExecutorFactory,
	ingestedDataReader AiAgentUsecaseIngestedDataReader,
	dataModelUsecase AiAgentUsecaseDataModelUsecase,
	caseReviewFileRepository caseReviewFileRepository,
	blobRepository repositories.BlobRepository,
) AiAgentUsecase {
	return AiAgentUsecase{
		enforceSecurity:          enforceSecurity,
		repository:               repository,
		inboxReader:              inboxReader,
		executorFactory:          executorFactory,
		ingestedDataReader:       ingestedDataReader,
		dataModelUsecase:         dataModelUsecase,
		caseReviewFileRepository: caseReviewFileRepository,
		blobRepository:           blobRepository,
	}
}

func (uc *AiAgentUsecase) GetClient(ctx context.Context) (*openai.Client, error) {
	uc.mu.Lock()
	defer uc.mu.Unlock()
	if uc.client == nil {
		client := openai.NewClient()
		uc.client = &client
	}
	return uc.client, nil
}

func (uc *AiAgentUsecase) GetCaseDataZip(ctx context.Context, caseId string) (io.Reader, error) {
	caseDtos, relatedDataPerClient, err := uc.getCaseDataWithPermissions(ctx, caseId)
	if err != nil {
		return nil, errors.Wrap(err, "could not get case data")
	}
	writeMap := map[string]any{
		"case.json":          caseDtos.case_,
		"case_events.json":   caseDtos.events,
		"decisions.json":     caseDtos.decisions,
		"data_model.json":    caseDtos.dataModel,
		"pivot_objects.json": caseDtos.pivotData,
	}

	pr, pw := io.Pipe()

	// Start writing zip archive in a goroutine
	go func() {
		zipw := zip.NewWriter(pw)
		defer func() {
			// Close in reverse order
			if r := recover(); r != nil {
				logger := utils.LoggerFromContext(ctx)
				logger.ErrorContext(ctx, "panic while writing zip archive", "error", r)
			}
			zipw.Close()
			pw.Close()
		}()

		for fileName, data := range writeMap {
			f, err := zipw.Create(fileName)
			if err != nil {
				pw.CloseWithError(errors.Wrapf(err, "could not create %s in zip", fileName))
				return
			}
			if err := json.NewEncoder(f).Encode(data); err != nil {
				pw.CloseWithError(errors.Wrapf(err, "could not write %s to zip", fileName))
				return
			}
		}

		for pivotObjectStr, data := range relatedDataPerClient.Data {
			pivotObjectFolder := fmt.Sprintf("related_data/%s/", pivotObjectStr)

			fileStr := pivotObjectFolder + "related_cases.json"
			f, err := zipw.Create(fileStr)
			if err != nil {
				pw.CloseWithError(errors.Wrapf(err, "could not create %s in zip", fileStr))
				return
			}
			if err := json.NewEncoder(f).Encode(data.RelatedCases); err != nil {
				pw.CloseWithError(errors.Wrapf(err, "could not write %s to zip", fileStr))
				return
			}

			for tableName, objects := range data.IngestedData {
				if len(objects.Data) == 0 {
					continue
				}
				fileStr := pivotObjectFolder + tableName + ".csv"
				f, err := zipw.Create(fileStr)
				if err != nil {
					pw.CloseWithError(errors.Wrapf(err, "could not create %s in zip", fileStr))
					return
				}
				csvFile := csv.NewWriter(f)
				if err := agent_dto.WriteClientDataToCsv(objects.Data, csvFile); err != nil {
					pw.CloseWithError(errors.Wrapf(err, "could not write %s to zip", fileStr))
					return
				}
			}
		}
	}()

	return pr, nil
}

func readPrompt(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", errors.Wrapf(err, "could not open prompt file %s", path)
	}
	defer file.Close()

	promptBytes, err := io.ReadAll(file)
	if err != nil {
		return "", errors.Wrapf(err, "could not read prompt file %s", path)
	}
	return string(promptBytes), nil
}

// Mutates the input (variadic)previousContents by appending the new input content and any genereated output, and returns them for convenience.
// "previous" returned by the method can be reused as input for the next call to generateContent in order to continue the conversation.
func (uc *AiAgentUsecase) generateContent(
	ctx context.Context,
	client *openai.Client,
	promptPath string,
	data map[string]any,
	generateContentConfig *GenerateContentConfig,
	previousContents ...openai.ChatCompletionMessageParamUnion,
) (out GenerateContentResult, err error) {
	prompt, err := readPrompt(promptPath)
	if err != nil {
		return out, err
	}

	// Load model configuration on each call
	modelConfig, err := models.LoadAiAgentModelConfig("prompts/ai_agent_models.json")
	if err != nil {
		return out, errors.Wrap(err, "could not load AI agent model configuration")
	}

	// Get the appropriate model for this prompt
	model := modelConfig.GetModelForPrompt(promptPath)

	logger := utils.LoggerFromContext(ctx)
	logger.InfoContext(ctx, "using model for prompt",
		"prompt", promptPath,
		"model", model)

	marshalledMap := make(map[string]string)
	for k, v := range data {
		if printer, ok := v.(agent_dto.AgentPrinter); ok {
			marshalledMap[k], err = printer.PrintForAgent()
			if err != nil {
				return out, errors.Wrapf(err, "could not print %s", k)
			}
		} else {
			b, err := json.Marshal(v)
			if err != nil {
				return out, errors.Wrapf(err, "could not marshal %s", k)
			}
			marshalledMap[k] = string(b)
		}
	}

	t, err := template.New(promptPath).Parse(prompt)
	if err != nil {
		return out, errors.Wrapf(err, "could not parse template %s", promptPath)
	}
	buf := bytes.Buffer{}
	err = t.Execute(&buf, marshalledMap)
	if err != nil {
		return out, errors.Wrap(err, "could not execute template")
	}

	// Build messages array
	messages := make([]openai.ChatCompletionMessageParamUnion, 0, len(previousContents)+2)

	// Add system message if provided
	if generateContentConfig.SystemInstruction != "" {
		messages = append(messages, openai.SystemMessage(
			generateContentConfig.SystemInstruction))
	}

	// Add previous messages
	messages = append(messages, previousContents...)

	// Add current prompt
	messages = append(messages, openai.UserMessage(buf.String()))

	// Create chat completion request
	chatService := client.Chat.Completions
	params := openai.ChatCompletionNewParams{
		Model:    model,
		Messages: messages,
	}

	// Add response format if specified
	if generateContentConfig.ResponseMIMEType == "application/json" {
		if generateContentConfig.ResponseSchema != nil {
			// Use JSON schema format for structured output
			params.ResponseFormat = openai.ChatCompletionNewParamsResponseFormatUnion{
				OfJSONSchema: &openai.ResponseFormatJSONSchemaParam{
					JSONSchema: *generateContentConfig.ResponseSchema,
				},
			}
		} else {
			// Use simple JSON object format
			params.ResponseFormat = openai.ChatCompletionNewParamsResponseFormatUnion{
				OfJSONObject: &openai.ResponseFormatJSONObjectParam{},
			}
		}
	}

	// Add tools if specified
	if len(generateContentConfig.Tools) > 0 {
		params.Tools = generateContentConfig.Tools
	}

	// Add tool choice if specified
	if generateContentConfig.ToolChoice != (openai.ChatCompletionToolChoiceOptionUnionParam{}) {
		params.ToolChoice = generateContentConfig.ToolChoice
	}

	// Add labels for billing attribution
	if generateContentConfig.Labels != nil {
		// OpenAI doesn't have direct labels support like GenAI, but we can add metadata
		// This would need to be handled differently in production
	}

	result, err := chatService.New(ctx, params)
	if err != nil {
		return out, err
	}
	if len(result.Choices) == 0 {
		return out, errors.New("no response from OpenAI")
	}

	choice := result.Choices[0]
	if choice.Message.Content == "" {
		return out, errors.New("no content in response from OpenAI")
	}

	// Extract text content
	text := choice.Message.Content

	// Extract tool calls (equivalent to "thought" in GenAI)
	thought := ""
	if len(choice.Message.ToolCalls) > 0 {
		for _, toolCall := range choice.Message.ToolCalls {
			if toolCall.Function.Arguments != "" {
				thought += toolCall.Function.Arguments
			}
		}
	}

	logger.InfoContext(ctx, "content detail",
		"prompt", promptPath,
		"model", model,
		"len_content", len(choice.Message.Content),
		"len_tool_calls", len(choice.Message.ToolCalls),
	)

	// Convert the response message to the format expected by previous contents
	responseMessage := openai.AssistantMessage(text)
	previous := append(previousContents, responseMessage)

	return GenerateContentResult{
		Text:     text,
		Thought:  thought,
		Previous: previous,
	}, nil
}

type GenerateContentConfig struct {
	SystemInstruction string
	ResponseMIMEType  string
	ResponseSchema    *openai.ResponseFormatJSONSchemaJSONSchemaParam
	Tools             []openai.ChatCompletionToolParam
	ToolChoice        openai.ChatCompletionToolChoiceOptionUnionParam
	Labels            map[string]string
}

type GenerateContentResult struct {
	Text     string
	Thought  string
	Previous []openai.ChatCompletionMessageParamUnion
}

// returns a slice of 0 or 1 case review dto, the most recent one.
func (uc *AiAgentUsecase) getMostRecentCaseReview(ctx context.Context, caseId string) ([]agent_dto.AiCaseReviewDto, error) {
	exec := uc.executorFactory.NewExecutor()
	c, err := uc.repository.GetCaseById(ctx, exec, caseId)
	if err != nil {
		return nil, err
	}

	inboxes, err := uc.inboxReader.ListInboxes(ctx, exec, c.OrganizationId, false)
	if err != nil {
		return nil,
			errors.Wrap(err, "failed to list available inboxes in AiAgentUsecase")
	}
	availableInboxIds := make([]uuid.UUID, len(inboxes))
	for i, inbox := range inboxes {
		availableInboxIds[i] = inbox.Id
	}
	if err := uc.enforceSecurity.ReadOrUpdateCase(c.GetMetadata(), availableInboxIds); err != nil {
		return nil, err
	}

	caseIdUuid, err := uuid.Parse(caseId)
	if err != nil {
		return nil, errors.Wrap(err, "could not parse case id")
	}
	existingCaseReviewFiles, err := uc.caseReviewFileRepository.ListCaseReviewFiles(ctx, exec, caseIdUuid)
	if err != nil {
		return nil, errors.Wrap(err, "could not list case review files")
	}

	if len(existingCaseReviewFiles) == 0 {
		return nil, nil
	}

	blob, err := uc.blobRepository.GetBlob(ctx, existingCaseReviewFiles[0].BucketName,
		existingCaseReviewFiles[0].FileReference)
	if err != nil {
		return nil, errors.Wrap(err, "could not get case review file")
	}
	reviewDto, err := agent_dto.UnmarshalCaseReviewDto(
		existingCaseReviewFiles[0].DtoVersion, blob.ReadCloser)
	if err != nil {
		return nil, errors.Wrap(err, "could not unmarshal case review file")
	}

	return []agent_dto.AiCaseReviewDto{reviewDto}, nil
}

func (uc *AiAgentUsecase) CreateCaseReview(ctx context.Context, caseId string) (agent_dto.AiCaseReviewDto, error) {
	existingReviewDtos, err := uc.getMostRecentCaseReview(ctx, caseId)
	if err != nil {
		return nil, errors.Wrap(err, "could not get case reviews")
	}
	if len(existingReviewDtos) > 0 {
		return existingReviewDtos[0], nil
	}

	client, err := uc.GetClient(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "could not create OpenAI client")
	}

	caseData, relatedDataPerClient, err := uc.getCaseDataWithPermissions(ctx, caseId)
	if err != nil {
		return nil, errors.Wrap(err, "could not get case data")
	}

	var clientActivityDescription string
	clientActivityDescription, err = readPrompt(fmt.Sprintf("prompts/org_desc/%s.md", caseData.organizationId))
	if err != nil {
		logger := utils.LoggerFromContext(ctx)
		logger.ErrorContext(ctx, "could not read organization description", "error", err)
		clientActivityDescription = "placeholder"
	}

	systemInstruction, err := readPrompt("prompts/system.md")
	if err != nil {
		logger := utils.LoggerFromContext(ctx)
		logger.ErrorContext(ctx, "could not read system instruction", "error", err)
		systemInstruction = "You are a compliance officer or fraud analyst. You are given a case and you need to review it step by step. Reply factually to instructions in markdown format."
	}

	// Data model summary
	dataModelSummaryResult, err := uc.generateContent(ctx,
		client,
		"prompts/case_review/data_model_summary.md",
		map[string]any{
			"data_model": caseData.dataModelDto,
		},
		&GenerateContentConfig{
			SystemInstruction: systemInstruction,
		},
	)
	if err != nil {
		return nil, errors.Wrap(err,
			"could not generate data model summary")
	}
	dataModelSummary := dataModelSummaryResult.Text
	previousContents := dataModelSummaryResult.Previous

	logger := utils.LoggerFromContext(ctx)
	logger.DebugContext(ctx, "================================ Data model summary ================================")
	logger.DebugContext(ctx, "Data model summary: "+dataModelSummary)
	if dataModelSummaryResult.Thought != "" {
		logger.DebugContext(ctx, "Data model summary thought: "+dataModelSummaryResult.Thought)
	}

	// Data model object field read options
	// Here, we implicitly distinguish between "transaction" tables (based on the presence of "many" rows for a given customer)
	// and other tables (where we can afford to read all fields)
	var fieldsToReadPerTable map[string][]string
	allPresentTables := make(map[string]bool)
	for _, clientData := range relatedDataPerClient.Data {
		for tableName := range clientData.IngestedData {
			allPresentTables[tableName] = true
		}
	}

	tablesWithLargRowNbs := make(map[string][]string)
	allTables := caseData.dataModel.Tables
	for tableName := range allPresentTables {
		if someClientHasManyRowsForTable(relatedDataPerClient.Data, tableName) {
			tablesWithLargRowNbs[tableName] = allTables[tableName].FieldNames()
		}
	}
	tableNamesWithLargRowNbs := pure_utils.Keys(tablesWithLargRowNbs)

	for customerKey, clientData := range relatedDataPerClient.Data {
		objectTables := clientData.IngestedData
		if objectTables != nil {
			// generate the map of fields to read for every table, but only once.
			if fieldsToReadPerTable == nil {
				// Build the JSON schema for field selection
				properties := make(map[string]interface{}, len(tablesWithLargRowNbs))
				for tableName, fields := range tablesWithLargRowNbs {
					properties[tableName] = map[string]interface{}{
						"type": "array",
						"items": map[string]interface{}{
							"type": "string",
							"enum": fields,
						},
					}
				}

				dataModelObjectFieldReadOptionsResult, err := uc.generateContent(
					ctx,
					client,
					"prompts/case_review/data_model_object_field_read_options.md",
					map[string]any{
						"data_model_table_names": tableNamesWithLargRowNbs,
					},
					&GenerateContentConfig{
						ResponseMIMEType: "application/json",
						ResponseSchema: &openai.ResponseFormatJSONSchemaJSONSchemaParam{
							Name:        "field_selection",
							Description: openai.String("List of fields, on a table per table basis, that should be read from ingested data for high-volume tables and considered for transaction analysis"),
							Schema: map[string]interface{}{
								"type":       "object",
								"properties": properties,
							},
						},
						SystemInstruction: systemInstruction,
					},
					previousContents...,
				)
				if err != nil {
					return nil, errors.Wrap(err,
						"could not generate data model object field read options")
				}
				dataModelObjectFieldReadOptions := dataModelObjectFieldReadOptionsResult.Text
				logger.DebugContext(ctx, "================================ Data model object field read options ================================")
				logger.DebugContext(ctx, "Data model object field read options: "+dataModelObjectFieldReadOptions)
				if dataModelObjectFieldReadOptionsResult.Thought != "" {
					logger.DebugContext(ctx, "Data model object field read options thought: "+
						dataModelObjectFieldReadOptionsResult.Thought)
				}
				fieldsToReadPerTable = make(map[string][]string)
				if err := json.Unmarshal([]byte(dataModelObjectFieldReadOptions), &fieldsToReadPerTable); err != nil {
					return nil, errors.Wrap(err,
						"could not unmarshal data model object field read options")
				}
			}

			for tableName, fieldsToRead := range fieldsToReadPerTable {
				// Reuse original read options, just adapt the number of rows to read and the fields to consider
				fieldFilteredObjects, _, _, err := uc.ingestedDataReader.ReadIngestedClientObjects(
					ctx,
					caseData.organizationId,
					tableName,
					models.ClientDataListRequestBody{
						ExplorationOptions: objectTables[tableName].ReadOptions,
						Limit:              500,
					},
					fieldsToRead...,
				)
				if err != nil {
					return nil, errors.Wrapf(err,
						"could not read ingested client objects for %s", tableName)
				}
				// then, update the ingested data for this pivot object/table combination with the new filtered data
				relatedDataPerClient.Data[customerKey].IngestedData[tableName] = agent_dto.IngestedDataResult{
					Data:        fieldFilteredObjects,
					ReadOptions: objectTables[tableName].ReadOptions,
				}
			}

		}
	}

	// Rules definitions review
	rulesDefinitionsReviewResult, err := uc.generateContent(ctx,
		client,
		"prompts/case_review/rule_definitions.md",
		map[string]any{
			"decisions":            caseData.decisions,
			"activity_description": clientActivityDescription,
		},
		&GenerateContentConfig{
			SystemInstruction: systemInstruction,
		},
	)
	rulesDefinitionsReview := rulesDefinitionsReviewResult.Text
	if err != nil {
		return nil, errors.Wrap(err,
			"could not generate rules definitions review")
	}
	logger.DebugContext(ctx, "================================ Rules definitions review ================================")
	logger.DebugContext(ctx, "Rules definitions review: "+rulesDefinitionsReview)
	if rulesDefinitionsReviewResult.Thought != "" {
		logger.DebugContext(ctx, "Rules definitions review thought: "+rulesDefinitionsReviewResult.Thought)
	}

	// Rule thresholds
	ruleThresholdsResult, err := uc.generateContent(ctx,
		client,
		"prompts/case_review/rule_threshold_values.md",
		map[string]any{
			"decisions": caseData.decisions,
		},
		&GenerateContentConfig{
			SystemInstruction: systemInstruction,
		},
	)
	ruleThresholds := ruleThresholdsResult.Text
	if err != nil {
		return nil, errors.Wrap(err, "could not generate rule thresholds")
	}
	logger.DebugContext(ctx, "================================ Rule thresholds ================================")
	logger.DebugContext(ctx, "Rule thresholds: "+ruleThresholds)
	if ruleThresholdsResult.Thought != "" {
		logger.DebugContext(ctx, "Rule thresholds thought: "+ruleThresholdsResult.Thought)
	}

	// Finally, we can generate the case review
	caseReviewResult, err := uc.generateContent(
		ctx,
		client,
		"prompts/case_review/case_review.md",
		map[string]any{
			"case_detail":        caseData.case_,
			"case_events":        caseData.events,
			"decisions":          caseData.decisions,
			"data_model_summary": dataModelSummary,
			"pivot_objects":      caseData.pivotData,
			"previous_cases":     relatedDataPerClient,
			"rules_summary":      rulesDefinitionsReview,
			"rule_thresholds":    ruleThresholds,
		},
		&GenerateContentConfig{
			SystemInstruction: systemInstruction,
		},
	)
	caseReview := caseReviewResult.Text
	if err != nil {
		return nil, errors.Wrap(err, "could not generate case review")
	}
	logger.DebugContext(ctx, "================================ Full case review ================================")
	logger.DebugContext(ctx, "Full case review: "+caseReview)
	if caseReviewResult.Thought != "" {
		logger.DebugContext(ctx, "Full case review thought: "+caseReviewResult.Thought)
	}

	// Finally, sanity check the resulting case review using a judgement prompt
	sanityCheckResult, err := uc.generateContent(ctx,
		client,
		"prompts/case_review/sanity_check.md",
		map[string]any{
			"case_detail":        caseData.case_,
			"case_events":        caseData.events,
			"decisions":          caseData.decisions,
			"data_model_summary": dataModelSummary,
			"pivot_objects":      caseData.pivotData,
			"previous_cases":     relatedDataPerClient,
			"rules_summary":      rulesDefinitionsReview,
			"rule_thresholds":    ruleThresholds,
			"case_review":        caseReview,
		},
		&GenerateContentConfig{
			ResponseMIMEType: "application/json",
			ResponseSchema: &openai.ResponseFormatJSONSchemaJSONSchemaParam{
				Name:        "sanity_check",
				Description: openai.String("Output of the sanity check and detailed justification"),
				Schema: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"ok": map[string]interface{}{
							"type":        "boolean",
							"description": "Whether the case review is ok or not",
						},
						"justification": map[string]interface{}{
							"type":        "string",
							"description": "Detailed justification for the sanity check, only in the case of a negative answer",
						},
					},
					"required": []string{"ok"},
				},
			},
			SystemInstruction: systemInstruction,
		},
	)
	sanityCheck := sanityCheckResult.Text
	if err != nil {
		return nil, errors.Wrap(err, "could not generate sanity check")
	}
	logger.DebugContext(ctx, "================================ Sanity check ================================")
	logger.DebugContext(ctx, "Sanity check: "+sanityCheck)
	if sanityCheckResult.Thought != "" {
		logger.DebugContext(ctx, "Sanity check thought: "+sanityCheckResult.Thought)
	}

	var sanityCheckOutput struct {
		Ok            bool   `json:"ok"`
		Justification string `json:"justification"`
	}
	if err := json.Unmarshal([]byte(sanityCheckResult.Text), &sanityCheckOutput); err != nil {
		return nil, errors.Wrap(err, "could not unmarshal sanity check")
	}

	if sanityCheckOutput.Ok {
		return agent_dto.CaseReviewV1{
			Ok:      sanityCheckOutput.Ok,
			Output:  caseReviewResult.Text,
			Thought: caseReviewResult.Thought,
		}, nil
	}
	return agent_dto.CaseReviewV1{
		Ok:          false,
		Output:      caseReview,
		SanityCheck: sanityCheckResult.Text,
		Thought:     caseReviewResult.Thought,
	}, nil
}

func (uc *AiAgentUsecase) getCaseDataWithPermissions(ctx context.Context, caseId string) (caseData, agent_dto.CasePivotDataByPivot, error) {
	exec := uc.executorFactory.NewExecutor()
	c, err := uc.repository.GetCaseById(ctx, exec, caseId)
	if err != nil {
		return caseData{}, agent_dto.CasePivotDataByPivot{}, err
	}

	inboxes, err := uc.inboxReader.ListInboxes(ctx, exec, c.OrganizationId, false)
	if err != nil {
		return caseData{}, agent_dto.CasePivotDataByPivot{},
			errors.Wrap(err, "failed to list available inboxes in usecase")
	}
	availableInboxIds := make([]uuid.UUID, len(inboxes))
	for i, inbox := range inboxes {
		availableInboxIds[i] = inbox.Id
	}

	if err := uc.enforceSecurity.ReadOrUpdateCase(c.GetMetadata(), availableInboxIds); err != nil {
		return caseData{}, agent_dto.CasePivotDataByPivot{}, err
	}

	tags, err := uc.repository.ListOrganizationTags(ctx, exec, c.OrganizationId, models.TagTargetCase, false)
	if err != nil {
		return caseData{}, agent_dto.CasePivotDataByPivot{},
			errors.Wrap(err, "could not retrieve tags for case")
	}
	caseEvents, err := uc.repository.ListCaseEvents(ctx, exec, caseId)
	if err != nil {
		return caseData{}, agent_dto.CasePivotDataByPivot{},
			errors.Wrap(err, "could not retrieve case events")
	}
	users, err := uc.repository.ListUsers(ctx, exec, &c.OrganizationId)
	if err != nil {
		return caseData{}, agent_dto.CasePivotDataByPivot{},
			errors.Wrap(err, "could not retrieve users for case events")
	}
	caseEventsDto := make([]agent_dto.CaseEvent, len(caseEvents))
	for i := range caseEvents {
		caseEventsDto[i] = agent_dto.AdaptCaseEventDto(caseEvents[i], users)
	}

	decisions, err := uc.repository.DecisionsByCaseId(ctx, exec, c.OrganizationId, caseId)
	if err != nil {
		return caseData{}, agent_dto.CasePivotDataByPivot{},
			errors.Wrap(err, "could not retrieve case decisions")
	}
	decisionDtos := make([]agent_dto.Decision, len(decisions))
	for i := range decisions {
		iteration, err := uc.repository.GetScenarioIteration(ctx, exec,
			decisions[i].Decision.ScenarioIterationId.String())
		if err != nil {
			return caseData{}, agent_dto.CasePivotDataByPivot{}, errors.Wrapf(err,
				"could not retrieve scenario for decision %s", decisions[i].DecisionId)
		}
		rules, err := uc.repository.ListRulesByIterationId(ctx, exec,
			decisions[i].Decision.ScenarioIterationId.String())
		if err != nil {
			return caseData{}, agent_dto.CasePivotDataByPivot{}, errors.Wrapf(err,
				"could not retrieve rules for decision %s", decisions[i].DecisionId)
		}
		screenings, err := uc.repository.ListScreeningsForDecision(ctx, exec, decisions[i].DecisionId.String(), true)
		if err != nil {
			return caseData{}, agent_dto.CasePivotDataByPivot{}, errors.Wrapf(err,
				"could not retrieve screenings for decision %s", decisions[i].DecisionId)
		}
		decisionDtos[i] = agent_dto.AdaptDecision(decisions[i].Decision, iteration,
			decisions[i].RuleExecutions, rules, screenings)
	}

	dataModel, err := uc.dataModelUsecase.GetDataModel(ctx, c.OrganizationId, models.DataModelReadOptions{
		IncludeEnums: true, IncludeNavigationOptions: true,
	})
	if err != nil {
		return caseData{}, agent_dto.CasePivotDataByPivot{},
			errors.Wrap(err, "could not retrieve data model")
	}

	pivotValues, err := uc.repository.DecisionPivotValuesByCase(ctx, exec, caseId)
	if err != nil {
		return caseData{}, agent_dto.CasePivotDataByPivot{}, err
	}
	pivotObjects, err := uc.ingestedDataReader.ReadPivotObjectsFromValues(ctx, c.OrganizationId, pivotValues)
	if err != nil {
		return caseData{}, agent_dto.CasePivotDataByPivot{},
			errors.Wrap(err, "could not read pivot objects from values")
	}
	pivotObjectDtos, err := pure_utils.MapErr(pivotObjects, agent_dto.AdaptPivotObjectDto)
	if err != nil {
		return caseData{}, agent_dto.CasePivotDataByPivot{},
			errors.Wrap(err, "could not adapt pivot objects to DTOs")
	}

	relatedDataPerClient := agent_dto.CasePivotDataByPivot{
		Data: make(map[string]agent_dto.CasePivotObjectData),
	}

	for _, pivotObject := range pivotObjects {
		pivotObjectData := agent_dto.CasePivotObjectData{
			IngestedData: make(map[string]agent_dto.IngestedDataResult, 10),
			RelatedCases: make([]agent_dto.CaseWithDecisions, 0, 10),
		}

		previousCases, err := uc.repository.GetCasesWithPivotValue(ctx, exec,
			c.OrganizationId, pivotObject.PivotValue)
		if err != nil {
			return caseData{}, agent_dto.CasePivotDataByPivot{}, err
		}
		relatedCases := make([]agent_dto.CaseWithDecisions, 0, 10)
		for _, previousCase := range previousCases {
			if previousCase.Id == c.Id {
				// skip the current case, we don't want to include it in the related cases
				continue
			}

			decisions, err := uc.repository.DecisionsByCaseId(ctx, exec, c.OrganizationId, previousCase.Id)
			if err != nil {
				return caseData{}, agent_dto.CasePivotDataByPivot{}, errors.Wrapf(err,
					"could not retrieve decisions for previous case %s", previousCase.Id)
			}
			previousCase.Decisions = decisions
			events, err := uc.repository.ListCaseEvents(ctx, exec, previousCase.Id)
			if err != nil {
				return caseData{}, agent_dto.CasePivotDataByPivot{}, err
			}
			previousCase.Events = events

			// don't add rule executions because we don't care for previous cases rule exec details
			rc, err := agent_dto.AdaptCaseWithDecisionsDto(
				previousCase,
				tags,
				inboxes,
				nil,
				users,
				func(scenarioIterationId string) (models.ScenarioIteration, error) {
					return uc.repository.GetScenarioIteration(ctx, exec, scenarioIterationId)
				},
				func(decisionId string) ([]models.ScreeningWithMatches, error) {
					return uc.repository.ListScreeningsForDecision(ctx, exec, decisionId, true)
				},
			)
			if err != nil {
				return caseData{}, agent_dto.CasePivotDataByPivot{}, errors.Wrapf(err,
					"could not adapt case with decisions for previous case %s", previousCase.Id)
			}
			relatedCases = append(relatedCases, rc)
		}
		pivotObjectData.RelatedCases = relatedCases

		navigationOptions := dataModel.Tables[pivotObject.PivotObjectName].NavigationOptions
		for _, navOption := range navigationOptions {
			if _, found := pivotObjectData.IngestedData[navOption.TargetTableName]; found {
				// If we already have data for this target table, skip it
				continue
			}
			sourceFieldValue, ok := pivotObject.PivotObjectData.Data[navOption.SourceFieldName]
			if !ok {
				continue
			}
			sourceFieldValueStr, ok := sourceFieldValue.(string)
			if !ok {
				continue
			}
			if navOption.Status == models.IndexStatusValid {
				readOptions := models.ExplorationOptions{
					SourceTableName:   pivotObject.PivotObjectName,
					FilterFieldName:   navOption.FilterFieldName,
					FilterFieldValue:  models.NewStringOrNumberFromString(sourceFieldValueStr),
					OrderingFieldName: navOption.OrderingFieldName,
				}
				objects, _, _, err := uc.ingestedDataReader.ReadIngestedClientObjects(ctx,
					c.OrganizationId, navOption.TargetTableName, models.ClientDataListRequestBody{
						ExplorationOptions: readOptions,
						Limit:              1000,
					})
				if err != nil {
					return caseData{}, agent_dto.CasePivotDataByPivot{}, errors.Wrapf(err,
						"could not read ingested client objects for %s with value %s",
						pivotObject.PivotObjectName, sourceFieldValueStr)
				}
				pivotObjectData.IngestedData[navOption.TargetTableName] = agent_dto.IngestedDataResult{
					Data:        objects,
					ReadOptions: readOptions,
				}
			}
		}

		relatedDataPerClient.Data[pivotObject.PivotObjectName+"_"+pivotObject.PivotValue] = pivotObjectData
	}

	return caseData{
		case_:          agent_dto.AdaptCaseDto(c, tags, inboxes, users),
		events:         caseEventsDto,
		decisions:      decisionDtos,
		dataModelDto:   agent_dto.AdaptDataModelDto(dataModel),
		dataModel:      dataModel,
		pivotData:      pivotObjectDtos,
		organizationId: c.OrganizationId,
	}, relatedDataPerClient, nil
}

type caseData struct {
	case_          agent_dto.Case
	events         []agent_dto.CaseEvent
	decisions      []agent_dto.Decision
	dataModelDto   agent_dto.DataModel
	dataModel      models.DataModel
	pivotData      []agent_dto.PivotObject
	organizationId string
}

func someClientHasManyRowsForTable(relatedDataPerClient map[string]agent_dto.CasePivotObjectData, tableName string) bool {
	for _, clientData := range relatedDataPerClient {
		if clientData.IngestedData[tableName].Data != nil {
			return len(clientData.IngestedData[tableName].Data) > HIGH_NB_ROWS_THRESHOLD
		}
	}
	return false
}
