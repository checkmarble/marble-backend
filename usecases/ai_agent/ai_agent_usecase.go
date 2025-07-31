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
	"github.com/checkmarble/marble-backend/infra"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/checkmarble/marble-backend/usecases/inboxes"
	"github.com/checkmarble/marble-backend/usecases/security"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/invopop/jsonschema"

	llm_adapter "github.com/checkmarble/llm-adapter"
	"github.com/checkmarble/llm-adapter/llms/aistudio"
	"github.com/checkmarble/llm-adapter/llms/openai"
	"github.com/google/uuid"
	"github.com/pkg/errors"
)

const HIGH_NB_ROWS_THRESHOLD = 100

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

type caseReviewTaskEnqueuer interface {
	EnqueueCaseReviewTask(
		ctx context.Context,
		tx repositories.Transaction,
		organizationId string,
		caseId string,
	) error
}

type AiAgentUsecase struct {
	enforceSecurity          security.EnforceSecurityCase
	repository               AiAgentUsecaseRepository
	inboxReader              inboxes.InboxReader
	executorFactory          executor_factory.ExecutorFactory
	transactionFactory       executor_factory.TransactionFactory
	ingestedDataReader       AiAgentUsecaseIngestedDataReader
	dataModelUsecase         AiAgentUsecaseDataModelUsecase
	caseReviewFileRepository caseReviewWorkerRepository
	blobRepository           repositories.BlobRepository
	caseReviewTaskEnqueuer   caseReviewTaskEnqueuer
	config                   infra.AIAgentConfiguration

	llmAdapter *llm_adapter.LlmAdapter
	mu         sync.Mutex
}

type sanityCheckOutput struct {
	Ok            bool   `json:"ok" jsonschema_description:"Whether the case review is ok or not" jsonschema_required:"true"`
	Justification string `json:"justification" jsonschema_description:"Detailed justification for the sanity check, only in the case of a negative answer" jsonschema_required:"false"`
}

func NewAiAgentUsecase(
	enforceSecurity security.EnforceSecurityCase,
	repository AiAgentUsecaseRepository,
	inboxReader inboxes.InboxReader,
	executorFactory executor_factory.ExecutorFactory,
	ingestedDataReader AiAgentUsecaseIngestedDataReader,
	dataModelUsecase AiAgentUsecaseDataModelUsecase,
	caseReviewFileRepository caseReviewWorkerRepository,
	blobRepository repositories.BlobRepository,
	caseReviewTaskEnqueuer caseReviewTaskEnqueuer,
	transactionFactory executor_factory.TransactionFactory,
	config infra.AIAgentConfiguration,
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
		caseReviewTaskEnqueuer:   caseReviewTaskEnqueuer,
		transactionFactory:       transactionFactory,
		config:                   config,
	}
}

func (uc *AiAgentUsecase) createOpenAIProvider() (llm_adapter.Llm, error) {
	opts := []openai.Opt{}
	if uc.config.MainAgentURL != "" {
		opts = append(opts, openai.WithBaseUrl(uc.config.MainAgentURL))
	}
	if uc.config.MainAgentKey != "" {
		opts = append(opts, openai.WithApiKey(uc.config.MainAgentKey))
	}

	provider, err := openai.New(opts...)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create OpenAI provider")
	}
	return provider, nil
}

func (uc *AiAgentUsecase) createAIStudioProvider() (llm_adapter.Llm, error) {
	opts := []aistudio.Opt{
		aistudio.WithBackend(uc.config.MainAgentBackend),
	}

	if uc.config.MainAgentKey != "" {
		opts = append(opts, aistudio.WithApiKey(uc.config.MainAgentKey))
	}
	if uc.config.MainAgentProject != "" {
		opts = append(opts, aistudio.WithProject(uc.config.MainAgentProject))
	}
	if uc.config.MainAgentLocation != "" {
		opts = append(opts, aistudio.WithLocation(uc.config.MainAgentLocation))
	}

	provider, err := aistudio.New(opts...)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create AI Studio provider")
	}
	return provider, nil
}

func (uc *AiAgentUsecase) GetClient(ctx context.Context) (*llm_adapter.LlmAdapter, error) {
	uc.mu.Lock()
	defer uc.mu.Unlock()

	if uc.llmAdapter != nil {
		return uc.llmAdapter, nil
	}

	// Create provider based on config
	var mainProvider llm_adapter.Llm
	var err error

	switch uc.config.MainAgentProviderType {
	case infra.AIAgentProviderTypeOpenAI:
		mainProvider, err = uc.createOpenAIProvider()
	case infra.AIAgentProviderTypeAIStudio:
		mainProvider, err = uc.createAIStudioProvider()
	default:
		return nil, errors.Errorf("unsupported provider type: %s", uc.config.MainAgentProviderType)
	}

	if err != nil {
		return nil, errors.Wrap(err, "failed to create LLM provider")
	}

	adapter, err := llm_adapter.New(llm_adapter.WithProvider("main", mainProvider))
	if err != nil {
		return nil, errors.Wrap(err, "failed to create LLM adapter")
	}

	uc.llmAdapter = adapter
	return uc.llmAdapter, nil
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

// Prepare the request for the LLM, the prompt comes from a file and need to be templated.
// The file contains some variables that are replaced by the data provided by the caller.
func (uc *AiAgentUsecase) prepareRequest(promptPath string, data map[string]any) (model string, prompt string, err error) {
	// Load prompt from file, do each time in case prompt configuration changes
	promptContent, err := readPrompt(promptPath)
	if err != nil {
		return "", "", errors.Wrap(err, "could not read prompt file")
	}

	// Load model configuration on each call
	// Give the possibility to change the prompt without reloading the application
	modelConfig, err := models.LoadAiAgentModelConfig("prompts/ai_agent_models.json", uc.config.MainAgentDefaultModel)
	if err != nil {
		return "", "", errors.Wrap(err, "could not load AI agent model configuration")
	}

	model = modelConfig.GetModelForPrompt(promptPath)

	// Build the prompt message with the data
	// Prepare the data for the template execution
	marshalledMap := make(map[string]string)
	for k, v := range data {
		if printer, ok := v.(agent_dto.AgentPrinter); ok {
			marshalledMap[k], err = printer.PrintForAgent()
			if err != nil {
				return "", "", errors.Wrapf(err, "could not print %s", k)
			}
		} else {
			b, err := json.Marshal(v)
			if err != nil {
				return "", "", errors.Wrapf(err, "could not marshal %s", k)
			}
			marshalledMap[k] = string(b)
		}
	}

	t, err := template.New(promptPath).Parse(promptContent)
	if err != nil {
		return "", "", errors.Wrapf(err, "could not parse template %s", promptPath)
	}
	buf := bytes.Buffer{}
	err = t.Execute(&buf, marshalledMap)
	if err != nil {
		return "", "", errors.Wrap(err, "could not execute template")
	}
	prompt = buf.String()

	return model, prompt, nil
}

func (uc *AiAgentUsecase) getCaseWithPermissions(ctx context.Context, caseId string) (models.Case, error) {
	exec := uc.executorFactory.NewExecutor()
	c, err := uc.repository.GetCaseById(ctx, exec, caseId)
	if err != nil {
		return models.Case{}, err
	}

	inboxes, err := uc.inboxReader.ListInboxes(ctx, exec, c.OrganizationId, false)
	if err != nil {
		return models.Case{},
			errors.Wrap(err, "failed to list available inboxes in AiAgentUsecase")
	}
	availableInboxIds := make([]uuid.UUID, len(inboxes))
	for i, inbox := range inboxes {
		availableInboxIds[i] = inbox.Id
	}
	if err := uc.enforceSecurity.ReadOrUpdateCase(c.GetMetadata(), availableInboxIds); err != nil {
		return models.Case{}, err
	}
	return c, nil
}

// returns a slice of 0 or 1 case review dto, the most recent one.
func (uc *AiAgentUsecase) getMostRecentCaseReview(ctx context.Context, caseId string) ([]agent_dto.AiCaseReviewDto, error) {
	exec := uc.executorFactory.NewExecutor()
	_, err := uc.getCaseWithPermissions(ctx, caseId)
	if err != nil {
		return nil, errors.Wrap(err, "could not get case with permissions")
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
	defer blob.ReadCloser.Close()

	reviewDto, err := agent_dto.UnmarshalCaseReviewDto(
		existingCaseReviewFiles[0].DtoVersion, blob.ReadCloser)
	if err != nil {
		return nil, errors.Wrap(err, "could not unmarshal case review file")
	}

	return []agent_dto.AiCaseReviewDto{reviewDto}, nil
}

// Returns a slice of 0 or 1 case review dto, the most recent one.
func (uc *AiAgentUsecase) GetCaseReview(ctx context.Context, caseId string) ([]agent_dto.AiCaseReviewDto, error) {
	_, err := uc.getCaseWithPermissions(ctx, caseId)
	if err != nil {
		return nil, err
	}

	existingReviewDtos, err := uc.getMostRecentCaseReview(ctx, caseId)
	if err != nil {
		return nil, errors.Wrap(err, "could not list case review dtos")
	}

	if len(existingReviewDtos) == 0 {
		return make([]agent_dto.AiCaseReviewDto, 0), nil
	}

	return existingReviewDtos[:1], nil
}

func (uc *AiAgentUsecase) EnqueueCreateCaseReview(ctx context.Context, caseId string) error {
	c, err := uc.getCaseWithPermissions(ctx, caseId)
	if err != nil {
		return err
	}

	return uc.transactionFactory.Transaction(ctx, func(tx repositories.Transaction) error {
		return uc.caseReviewTaskEnqueuer.EnqueueCaseReviewTask(ctx, tx, c.OrganizationId, caseId)
	})
}

// CreateCaseReviewSync performs a comprehensive AI-powered review of a case by analyzing
// case data, related information, and generating structured insights through multiple
// AI model interactions. The process involves several key steps:
// 1. Initialize AI client and gather case data with security validation
// 2. Generate data model summary to understand the case structure
// 3. Determine optimal fields to read for large tables to manage data volume
// 4. Analyze rule definitions and thresholds for context
// 5. Generate the main case review with all available information
// 6. Perform sanity check on the generated review for quality assurance
func (uc *AiAgentUsecase) CreateCaseReviewSync(ctx context.Context, caseId string) (agent_dto.AiCaseReviewDto, error) {
	logger := utils.LoggerFromContext(ctx)

	client, err := uc.GetClient(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "could not create ai client")
	}

	caseData, relatedDataPerClient, err := uc.getCaseDataWithPermissions(ctx, caseId)
	if err != nil {
		return nil, errors.Wrap(err, "could not get case data")
	}

	// Get Organization prompt from file if defined, this prompt gives more details about the organization
	var clientActivityDescription string
	clientActivityDescription, err = readPrompt(fmt.Sprintf("prompts/org_desc/%s.md", caseData.organizationId))
	if err != nil {
		logger.DebugContext(ctx, "could not read organization description", "error", err)
		clientActivityDescription = "placeholder"
	}

	// Define the system instruction for prompt
	systemInstruction, err := readPrompt("prompts/system.md")
	if err != nil {
		logger.DebugContext(ctx, "could not read system instruction", "error", err)
		systemInstruction = "You are a compliance officer or fraud analyst. You are given a case and you need to review it step by step. Reply factually to instructions in markdown format."
	}

	// Data model summary, create thread because the response will be used in next steps
	modelDataModelSummary, promptDataModelSummary, err := uc.prepareRequest(
		"prompts/case_review/data_model_summary.md", map[string]any{
			"data_model": caseData.dataModelDto,
		})
	if err != nil {
		return nil, errors.Wrap(err, "could not prepare data model summary request")
	}

	// Create the request with Thread for the next steps which needs the response
	requestDataModelSummary, err := llm_adapter.NewUntypedRequest().
		CreateThread().
		WithModel(modelDataModelSummary).
		WithInstruction(systemInstruction).
		WithText(llm_adapter.RoleUser, promptDataModelSummary).
		Do(ctx, client)
	if err != nil {
		return nil, errors.Wrap(err, "could not generate data model summary")
	}
	defer requestDataModelSummary.ThreadId.Clear()

	dataModelSummary, err := requestDataModelSummary.Get(0)
	if err != nil {
		return nil, errors.Wrap(err, "could not get data model summary")
	}

	logger.DebugContext(ctx, "================================ Data model summary ================================")
	logger.DebugContext(ctx, "Data model summary", "response", dataModelSummary)

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
				props := jsonschema.NewProperties()

				for tableName, fields := range tablesWithLargRowNbs {
					props.Set(tableName, &jsonschema.Schema{
						Type: "array",
						Items: &jsonschema.Schema{
							Type: "string",
							Enum: pure_utils.ToAnySlice(fields),
						},
					})
				}

				schema := jsonschema.Schema{
					Type:       "object",
					Properties: props,
				}

				modelDataModelObjectFieldReadOptions, promptDataModelObjectFieldReadOptions, err := uc.prepareRequest(
					"prompts/case_review/data_model_object_field_read_options.md",
					map[string]any{
						"data_model_table_names": tableNamesWithLargRowNbs,
					},
				)
				if err != nil {
					return nil, errors.Wrap(err, "could not prepare data model object field read options request")
				}

				requestDataModelObjectFieldReadOptions, err := llm_adapter.NewRequest[map[string][]string]().
					OverrideResponseSchema(schema).
					FromCandidate(requestDataModelSummary, 0).
					WithModel(modelDataModelObjectFieldReadOptions).
					WithInstruction(systemInstruction).
					WithText(llm_adapter.RoleUser, promptDataModelObjectFieldReadOptions).
					Do(ctx, client)
				if err != nil {
					return nil, errors.Wrap(err, "could not generate data model object field read options")
				}

				dataModelObjectFieldReadOptions, err :=
					requestDataModelObjectFieldReadOptions.Get(0)
				if err != nil {
					return nil, errors.Wrap(err, "could not get data model object field read options")
				}

				logger.DebugContext(ctx, "================================ Data model object field read options ================================")
				logger.DebugContext(ctx, "Data model object field read options",
					"response", dataModelObjectFieldReadOptions)

				fieldsToReadPerTable = dataModelObjectFieldReadOptions
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
	modelRulesDefinitions, promptRulesDefinitions, err := uc.prepareRequest(
		"prompts/case_review/rule_definitions.md",
		map[string]any{
			"decisions":            caseData.decisions,
			"activity_description": clientActivityDescription,
		},
	)
	if err != nil {
		return nil, errors.Wrap(err, "could not prepare rules definitions review request")
	}
	requestRulesDefinitionsReview, err := llm_adapter.NewUntypedRequest().
		WithModel(modelRulesDefinitions).
		WithInstruction(systemInstruction).
		WithText(llm_adapter.RoleUser, promptRulesDefinitions).
		Do(ctx, client)
	if err != nil {
		return nil, errors.Wrap(err, "could not generate rules definitions review")
	}
	rulesDefinitionsReview, err := requestRulesDefinitionsReview.Get(0)
	if err != nil {
		return nil, errors.Wrap(err, "could not get rules definitions review")
	}

	logger.DebugContext(ctx, "================================ Rules definitions review ================================")
	logger.DebugContext(ctx, "Rules definitions review", "response", rulesDefinitionsReview)

	// Rule thresholds
	modelRuleThresholds, promptRuleThresholds, err := uc.prepareRequest(
		"prompts/case_review/rule_threshold_values.md",
		map[string]any{
			"decisions": caseData.decisions,
		},
	)
	if err != nil {
		return nil, errors.Wrap(err, "could not prepare rule thresholds request")
	}
	requestRuleThresholds, err := llm_adapter.NewUntypedRequest().
		WithModel(modelRuleThresholds).
		WithInstruction(systemInstruction).
		WithText(llm_adapter.RoleUser, promptRuleThresholds).
		Do(ctx, client)
	if err != nil {
		return nil, errors.Wrap(err, "could not generate rule thresholds")
	}
	ruleThresholds, err := requestRuleThresholds.Get(0)
	if err != nil {
		return nil, errors.Wrap(err, "could not get rule thresholds")
	}

	logger.DebugContext(ctx, "================================ Rule thresholds ================================")
	logger.DebugContext(ctx, "Rule thresholds", "response", ruleThresholds)

	// Finally, we can generate the case review
	modelCaseReview, promptCaseReview, err := uc.prepareRequest(
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
	)
	if err != nil {
		return nil, errors.Wrap(err, "could not prepare case review request")
	}

	type caseReviewOutput struct {
		CaseReview string `json:"case_review" jsonschema_description:"The case review analysis including all the information needed to make a the review"`
		Proofs     []struct {
			Id          string `json:"id" jsonschema_description:"The id of the object used as proof, for organization data model you have an object_id"`
			Type        string `json:"type" jsonschema_description:"The type of the object used as proof, could be decision or case, for data model from organization, take the trigger_object_type"`
			IsDataModel bool   `json:"is_data_model" jsonschema_description:"Whether the proof is from organization data model. Decision and case are not data model"`
			Reason      string `json:"reason" jsonschema_description:"The reason of why this object was usefull for your review"`
		} `json:"proofs" jsonschema_description:"The proofs used to generate the case review"`
	}

	requestCaseReview, err := llm_adapter.NewRequest[caseReviewOutput]().
		WithModel(modelCaseReview).
		WithInstruction(systemInstruction).
		WithText(llm_adapter.RoleUser, promptCaseReview).
		Do(ctx, client)
	if err != nil {
		return nil, errors.Wrap(err, "could not generate case review")
	}
	caseReview, err := requestCaseReview.Get(0)
	if err != nil {
		return nil, errors.Wrap(err, "could not get case review")
	}

	logger.DebugContext(ctx, "================================ Full case review ================================")
	logger.DebugContext(ctx, "Full case review", "response", caseReview)

	// Finally, sanity check the resulting case review using a judgement prompt
	modelSanityCheck, promptSanityCheck, err := uc.prepareRequest(
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
	)
	if err != nil {
		return nil, errors.Wrap(err, "could not prepare sanity check request")
	}
	requestSanityCheck, err := llm_adapter.NewRequest[sanityCheckOutput]().
		WithModel(modelSanityCheck).
		WithInstruction(systemInstruction).
		WithText(llm_adapter.RoleUser, promptSanityCheck).
		Do(ctx, client)
	if err != nil {
		return nil, errors.Wrap(err, "could not generate sanity check")
	}
	sanityCheck, err := requestSanityCheck.Get(0)
	if err != nil {
		return nil, errors.Wrap(err, "could not get sanity check")
	}

	logger.DebugContext(ctx, "================================ Sanity check ================================")
	logger.DebugContext(ctx, "Sanity check", "response", sanityCheck)

	proofs := make([]agent_dto.CaseReviewProof, len(caseReview.Proofs))
	for i, proof := range caseReview.Proofs {
		proofs[i] = agent_dto.CaseReviewProof{
			Id:          proof.Id,
			Type:        proof.Type,
			IsDataModel: proof.IsDataModel,
			Reason:      proof.Reason,
		}
	}
	if sanityCheck.Ok {
		return agent_dto.CaseReviewV1{
			Ok:     sanityCheck.Ok,
			Output: caseReview.CaseReview,
			Proofs: proofs,
		}, nil
	}
	return agent_dto.CaseReviewV1{
		Ok:          false,
		Output:      caseReview.CaseReview,
		SanityCheck: sanityCheck.Justification,
		Proofs:      proofs,
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
