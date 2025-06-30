package usecases

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
	"github.com/pkg/errors"
	"google.golang.org/genai"
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
	enforceSecurity    security.EnforceSecurityCase
	repository         AiAgentUsecaseRepository
	inboxReader        inboxes.InboxReader
	executorFactory    executor_factory.ExecutorFactory
	ingestedDataReader AiAgentUsecaseIngestedDataReader
	dataModelUsecase   AiAgentUsecaseDataModelUsecase

	client    *genai.Client
	mu        sync.Mutex
	gcpRegion string
}

func NewAiAgentUsecase(
	enforceSecurity security.EnforceSecurityCase,
	repository AiAgentUsecaseRepository,
	inboxReader inboxes.InboxReader,
	executorFactory executor_factory.ExecutorFactory,
	ingestedDataReader AiAgentUsecaseIngestedDataReader,
	dataModelUsecase AiAgentUsecaseDataModelUsecase,
) AiAgentUsecase {
	return AiAgentUsecase{
		enforceSecurity:    enforceSecurity,
		repository:         repository,
		inboxReader:        inboxReader,
		executorFactory:    executorFactory,
		ingestedDataReader: ingestedDataReader,
		dataModelUsecase:   dataModelUsecase,

		// Some models are only available in the "global" region. Choose a proper region in production.
		gcpRegion: utils.GetEnv("VERTEX_AI_GCP_REGION", "global"),
	}
}

func (uc *AiAgentUsecase) GetClient(ctx context.Context) (*genai.Client, error) {
	uc.mu.Lock()
	defer uc.mu.Unlock()
	var err error
	if uc.client == nil {
		uc.client, err = genai.NewClient(ctx, &genai.ClientConfig{
			Location: uc.gcpRegion,
			Backend:  genai.BackendVertexAI,
		})
		if err != nil {
			return nil, errors.Wrap(err, "could not create GenAI client")
		}
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
	client *genai.Client,
	organizationId string,
	promptPath string,
	data map[string]any,
	generateContentConfig *genai.GenerateContentConfig,
	previousContents ...*genai.Content,
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

	// set the organization id in the labels for billing attribution
	if generateContentConfig.Labels == nil {
		generateContentConfig.Labels = make(map[string]string)
	}
	generateContentConfig.Labels["organization_id"] = organizationId
	prompt = buf.String()
	result, err := client.Models.GenerateContent(
		ctx,
		model,
		append(previousContents, genai.Text(prompt)...),
		generateContentConfig,
	)
	if err != nil {
		return out, err
	}
	if len(result.Candidates) == 0 {
		return out, errors.New("no response from GenAI")
	}
	if result.Candidates[0].Content == nil {
		return out, errors.New("no content in response from GenAI")
	}

	onlyTextParts := make([]string, 0, len(result.Candidates[0].Content.Parts))
	for _, part := range result.Candidates[0].Content.Parts {
		if !part.Thought && part.Text != "" {
			onlyTextParts = append(onlyTextParts, part.Text)
		}
	}
	thinkingTextParts := make([]string, 0, len(result.Candidates[0].Content.Parts))
	for _, part := range result.Candidates[0].Content.Parts {
		if part.Thought && part.Text != "" {
			thinkingTextParts = append(thinkingTextParts, part.Text)
		}
	}

	logger.InfoContext(ctx, "content detail",
		"prompt", promptPath,
		"model", model,
		"len", len(result.Candidates[0].Content.Parts),
		"len_filtered_text", len(onlyTextParts),
		"len_thinking_text", len(thinkingTextParts),
	)

	previous := append(previousContents, result.Candidates[0].Content)
	gatherText := ""
	for _, t := range onlyTextParts {
		gatherText += t
	}
	gatherThought := ""
	for _, t := range thinkingTextParts {
		gatherThought += t
	}

	return GenerateContentResult{
		Text:     gatherText,
		Thought:  gatherThought,
		Previous: previous,
	}, nil
}

type GenerateContentResult struct {
	Text     string
	Thought  string
	Previous []*genai.Content
}

func (uc *AiAgentUsecase) CreateCaseReview(ctx context.Context, caseId string) (string, error) {
	client, err := uc.GetClient(ctx)
	if err != nil {
		return "", errors.Wrap(err, "could not create GenAI client")
	}

	caseData, relatedDataPerClient, err := uc.getCaseDataWithPermissions(ctx, caseId)
	if err != nil {
		return "", errors.Wrap(err, "could not get case data")
	}

	var clientActivityDescription string
	clientActivityDescription, err = readPrompt(fmt.Sprintf("prompts/org_desc/%s.md", caseData.organizationId))
	if err != nil {
		logger := utils.LoggerFromContext(ctx)
		logger.ErrorContext(ctx, "could not read organization description", "error", err)
		clientActivityDescription = "placeholder"
	}

	// Data model summary
	dataModelSummaryResult, err := uc.generateContent(ctx,
		client,
		caseData.organizationId,
		"prompts/case_review/data_model_summary.md",
		map[string]any{
			"data_model": caseData.dataModelDto,
		},
		&genai.GenerateContentConfig{},
	)
	if err != nil {
		return "", errors.Wrap(err, "could not generate data model summary")
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
				properties := make(map[string]*genai.Schema, len(tablesWithLargRowNbs))
				for tableName, fields := range tablesWithLargRowNbs {
					properties[tableName] = &genai.Schema{
						Type:  "array",
						Items: &genai.Schema{Type: "string", Enum: fields},
					}
				}
				dataModelObjectFieldReadOptionsResult, err := uc.generateContent(
					ctx,
					client,
					caseData.organizationId,
					"prompts/case_review/data_model_object_field_read_options.md",
					map[string]any{
						"data_model_table_names": tableNamesWithLargRowNbs,
					},
					&genai.GenerateContentConfig{
						ResponseMIMEType: "application/json",
						ResponseSchema: &genai.Schema{
							Type:        "object",
							Description: "List of fields, on a table per table basis, that should be read from ingested data for high-volume tables and considered for transaction analysis",
							Properties:  properties,
						},
					},
					previousContents...,
				)
				if err != nil {
					return "", errors.Wrap(err, "could not generate data model object field read options")
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
					return "", errors.Wrap(err, "could not unmarshal data model object field read options")
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
					return "", errors.Wrapf(err, "could not read ingested client objects for %s", tableName)
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
		caseData.organizationId,
		"prompts/case_review/rule_definitions.md",
		map[string]any{
			"decisions":            caseData.decisions,
			"activity_description": clientActivityDescription,
		},
		&genai.GenerateContentConfig{
			Tools: []*genai.Tool{
				{GoogleSearch: &genai.GoogleSearch{}},
			},
		},
	)
	rulesDefinitionsReview := rulesDefinitionsReviewResult.Text
	if err != nil {
		return "", errors.Wrap(err, "could not generate rules definitions review")
	}
	logger.DebugContext(ctx, "================================ Rules definitions review ================================")
	logger.DebugContext(ctx, "Rules definitions review: "+rulesDefinitionsReview)
	if rulesDefinitionsReviewResult.Thought != "" {
		logger.DebugContext(ctx, "Rules definitions review thought: "+rulesDefinitionsReviewResult.Thought)
	}

	// Rule thresholds
	ruleThresholdsResult, err := uc.generateContent(ctx,
		client,
		caseData.organizationId,
		"prompts/case_review/rule_threshold_values.md",
		map[string]any{
			"decisions": caseData.decisions,
		},
		&genai.GenerateContentConfig{},
	)
	ruleThresholds := ruleThresholdsResult.Text
	if err != nil {
		return "", errors.Wrap(err, "could not generate rule thresholds")
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
		caseData.organizationId,
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
		&genai.GenerateContentConfig{
			Tools: []*genai.Tool{
				{GoogleSearch: &genai.GoogleSearch{}},
			},
		},
	)
	caseReview := caseReviewResult.Text
	if err != nil {
		return "", errors.Wrap(err, "could not generate case review")
	}
	logger.DebugContext(ctx, "================================ Full case review ================================")
	logger.DebugContext(ctx, "Full case review: "+caseReview)
	if caseReviewResult.Thought != "" {
		logger.DebugContext(ctx, "Full case review thought: "+caseReviewResult.Thought)
	}

	// Finally, sanity check the resulting case review using a judgement prompt
	sanityCheckResult, err := uc.generateContent(ctx,
		client,
		caseData.organizationId,
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
		&genai.GenerateContentConfig{
			ResponseMIMEType: "application/json",
			ResponseSchema: &genai.Schema{
				Type:        "object",
				Description: "Output of the sanity check and detailed justification",
				Properties: map[string]*genai.Schema{
					"ok": {
						Type:        "boolean",
						Description: "Whether the case review is ok or not",
					},
					"justification": {
						Type:        "string",
						Description: "Detailed justification for the sanity check, only in the case of a negative answer",
					},
				},
				Required: []string{"ok"},
			},
		},
	)
	sanityCheck := sanityCheckResult.Text
	if err != nil {
		return "", errors.Wrap(err, "could not generate sanity check")
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
		return "", errors.Wrap(err, "could not unmarshal sanity check")
	}

	if sanityCheckOutput.Ok {
		return caseReview, nil
	}
	return fmt.Sprintf("Review is ko: original review:%s\nsanity check output:%s", caseReview, sanityCheckOutput.Justification), nil
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
			decisions[i].Decision.ScenarioIterationId)
		if err != nil {
			return caseData{}, agent_dto.CasePivotDataByPivot{}, errors.Wrapf(err,
				"could not retrieve scenario for decision %s", decisions[i].DecisionId)
		}
		rules, err := uc.repository.ListRulesByIterationId(ctx, exec,
			decisions[i].Decision.ScenarioIterationId)
		if err != nil {
			return caseData{}, agent_dto.CasePivotDataByPivot{}, errors.Wrapf(err,
				"could not retrieve rules for decision %s", decisions[i].DecisionId)
		}
		screenings, err := uc.repository.ListScreeningsForDecision(ctx, exec, decisions[i].DecisionId, true)
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
