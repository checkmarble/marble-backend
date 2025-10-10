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
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/checkmarble/marble-backend/usecases/inboxes"
	"github.com/checkmarble/marble-backend/usecases/security"
	"github.com/checkmarble/marble-backend/utils"

	"github.com/checkmarble/llmberjack"
	"github.com/checkmarble/llmberjack/llms/aistudio"
	"github.com/checkmarble/llmberjack/llms/openai"
	"github.com/google/uuid"
	"github.com/pkg/errors"
)

type AiAgentUsecaseRepository interface {
	GetCaseById(ctx context.Context, exec repositories.Executor, caseId string) (models.Case, error)
	ListCaseEvents(ctx context.Context, exec repositories.Executor, caseId string) ([]models.CaseEvent, error)
	GetRuleById(ctx context.Context, exec repositories.Executor, ruleId string) (models.Rule, error)
	ListRulesByIterationId(ctx context.Context, exec repositories.Executor, iterationId string) ([]models.Rule, error)
	ListUsers(ctx context.Context, exec repositories.Executor, organizationIDFilter *string) ([]models.User, error)
	DecisionsByCaseIdFromCursor(
		ctx context.Context,
		exec repositories.Executor,
		req models.CaseDecisionsRequest,
	) ([]models.DecisionWithRulesAndScreeningsBaseInfo, bool, error)
	DecisionsWithRuleExecutionsByIds(
		ctx context.Context,
		exec repositories.Executor,
		decisionIds []string,
	) ([]models.DecisionWithRuleExecutions, error)
	DecisionPivotValuesByCase(ctx context.Context, exec repositories.Executor, caseId string) ([]models.PivotDataWithCount, error)
	GetCasesWithPivotValue(ctx context.Context, exec repositories.Executor,
		orgId, pivotValue string) ([]models.Case, error)
	ListOrganizationTags(ctx context.Context, exec repositories.Executor, organizationId string,
		target models.TagTarget, withCaseCount bool) ([]models.Tag, error)
	GetScenarioIteration(ctx context.Context, exec repositories.Executor, scenarioIterationId string,
		useCache bool) (models.ScenarioIteration, error)
	ListScreeningsForDecision(ctx context.Context, exec repositories.Executor, decisionId string,
		initialOnly bool) ([]models.ScreeningWithMatches, error)
	UpdateAiCaseReviewFeedback(
		ctx context.Context,
		exec repositories.Executor,
		reviewId uuid.UUID,
		feedback models.AiCaseReviewFeedback,
	) error
	GetCaseReviewById(ctx context.Context, exec repositories.Executor, reviewId uuid.UUID) (models.AiCaseReview, error)
	GetOrganizationById(ctx context.Context, exec repositories.Executor, organizationId string) (models.Organization, error)
	GetAiSetting(ctx context.Context, exec repositories.Executor, organizationId string) (*models.AiSetting, error)
	PutAiSetting(
		ctx context.Context,
		exec repositories.Executor,
		orgId string,
		setting models.UpsertAiSetting,
	) (models.AiSetting, error)
}

type AiAgentUsecaseCustomListRepository interface {
	AllCustomLists(ctx context.Context, exec repositories.Executor, organizationId string) ([]models.CustomList, error)
}

type AiAgentUsecaseCustomListUsecase interface {
	GetCustomLists(ctx context.Context, organizationId string) ([]models.CustomList, error)
}

type AiAgentUsecaseIngestedDataReader interface {
	GetIngestedObject(
		ctx context.Context,
		organizationId string,
		dataModel *models.DataModel,
		objectType string,
		uniqueFieldValue string,
		uniqueFieldName string,
	) ([]models.ClientObjectDetail, error)
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
	GetDataModel(ctx context.Context, organizationID string, options models.DataModelReadOptions,
		useCache bool) (models.DataModel, error)
}

type AiAgentUsecaseRuleUsecase interface {
	GetRule(ctx context.Context, ruleId string) (models.Rule, error)
}

type caseReviewTaskEnqueuer interface {
	EnqueueCaseReviewTask(
		ctx context.Context,
		tx repositories.Transaction,
		organizationId string,
		caseId uuid.UUID,
		aiCaseReviewId uuid.UUID,
	) error
}

type AiAgentUsecase struct {
	enforceSecurityCase         security.EnforceSecurityCase
	enforceSecurityOrganization security.EnforceSecurityOrganization
	repository                  AiAgentUsecaseRepository
	inboxReader                 inboxes.InboxReader
	executorFactory             executor_factory.ExecutorFactory
	transactionFactory          executor_factory.TransactionFactory
	ingestedDataReader          AiAgentUsecaseIngestedDataReader
	dataModelUsecase            AiAgentUsecaseDataModelUsecase
	ruleUsecase                 AiAgentUsecaseRuleUsecase
	customListUsecase           AiAgentUsecaseCustomListUsecase
	caseReviewFileRepository    caseReviewWorkerRepository
	blobRepository              repositories.BlobRepository
	caseReviewTaskEnqueuer      caseReviewTaskEnqueuer
	config                      infra.AIAgentConfiguration
	caseManagerBucketUrl        string

	caseReviewAdapter *llmberjack.Llmberjack
	enrichmentAdapter *llmberjack.Llmberjack
	mu                sync.Mutex
}

func NewAiAgentUsecase(
	enforceSecurityCase security.EnforceSecurityCase,
	enforceSecurityOrganization security.EnforceSecurityOrganization,
	repository AiAgentUsecaseRepository,
	inboxReader inboxes.InboxReader,
	executorFactory executor_factory.ExecutorFactory,
	ingestedDataReader AiAgentUsecaseIngestedDataReader,
	dataModelUsecase AiAgentUsecaseDataModelUsecase,
	ruleUsecase AiAgentUsecaseRuleUsecase,
	customListUsecase AiAgentUsecaseCustomListUsecase,
	caseReviewFileRepository caseReviewWorkerRepository,
	blobRepository repositories.BlobRepository,
	caseReviewTaskEnqueuer caseReviewTaskEnqueuer,
	transactionFactory executor_factory.TransactionFactory,
	config infra.AIAgentConfiguration,
	caseManagerBucketUrl string,
) AiAgentUsecase {
	return AiAgentUsecase{
		enforceSecurityCase:         enforceSecurityCase,
		enforceSecurityOrganization: enforceSecurityOrganization,
		repository:                  repository,
		inboxReader:                 inboxReader,
		executorFactory:             executorFactory,
		ingestedDataReader:          ingestedDataReader,
		dataModelUsecase:            dataModelUsecase,
		ruleUsecase:                 ruleUsecase,
		customListUsecase:           customListUsecase,
		caseReviewFileRepository:    caseReviewFileRepository,
		blobRepository:              blobRepository,
		caseReviewTaskEnqueuer:      caseReviewTaskEnqueuer,
		transactionFactory:          transactionFactory,
		config:                      config,
		caseManagerBucketUrl:        caseManagerBucketUrl,
	}
}

func (uc *AiAgentUsecase) createOpenAIProvider() (llmberjack.Llm, error) {
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

func (uc *AiAgentUsecase) createAIStudioProvider() (llmberjack.Llm, error) {
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

func (uc *AiAgentUsecase) GetClient(ctx context.Context) (*llmberjack.Llmberjack, error) {
	uc.mu.Lock()
	defer uc.mu.Unlock()

	if uc.caseReviewAdapter != nil {
		return uc.caseReviewAdapter, nil
	}

	// Create provider based on config
	var mainProvider llmberjack.Llm
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

	adapter, err := llmberjack.New(llmberjack.WithProvider("main", mainProvider))
	if err != nil {
		return nil, errors.Wrap(err, "failed to create LLM adapter")
	}

	uc.caseReviewAdapter = adapter
	return uc.caseReviewAdapter, nil
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

		for pivotObjectStr, ingestedData := range relatedDataPerClient.ingestedData {
			pivotObjectFolder := fmt.Sprintf("related_data/%s/", pivotObjectStr)

			fileStr := pivotObjectFolder + "related_cases.json"
			f, err := zipw.Create(fileStr)
			if err != nil {
				pw.CloseWithError(errors.Wrapf(err, "could not create %s in zip", fileStr))
				return
			}

			relatedCasesData, ok := relatedDataPerClient.relatedCases[pivotObjectStr]
			if ok {
				if err := json.NewEncoder(f).Encode(relatedCasesData); err != nil {
					pw.CloseWithError(errors.Wrapf(err, "could not write %s to zip", fileStr))
					return
				}
			}

			for tableName, objects := range ingestedData {
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
func preparePrompt(promptPath string, data map[string]any) (prompt string, err error) {
	// Load prompt from file, do each time in case prompt configuration changes
	promptContent, err := readPrompt(promptPath)
	if err != nil {
		return "", errors.Wrap(err, "could not read prompt file")
	}

	// Build the prompt message with the data
	// Prepare the data for the template execution
	marshalledMap := make(map[string]string)
	for k, v := range data {
		if printer, ok := v.(agent_dto.AgentPrinter); ok {
			marshalledMap[k], err = printer.PrintForAgent()
			if err != nil {
				return "", errors.Wrapf(err, "could not print %s", k)
			}
		} else {
			b, err := json.Marshal(v)
			if err != nil {
				return "", errors.Wrapf(err, "could not marshal %s", k)
			}
			marshalledMap[k] = string(b)
		}
	}

	t, err := template.New(promptPath).Funcs(templateFuncMap).Parse(promptContent)
	if err != nil {
		return "", errors.Wrapf(err, "could not parse template %s", promptPath)
	}
	buf := bytes.Buffer{}
	err = t.Execute(&buf, marshalledMap)
	if err != nil {
		return "", errors.Wrap(err, "could not execute template")
	}
	prompt = buf.String()

	return prompt, nil
}

// Call preparePrompt and complete the model with the model configuration
func (uc *AiAgentUsecase) preparePromptWithModel(promptPath string, data map[string]any) (model string, prompt string, err error) {
	// Load model configuration on each call
	// Give the possibility to change the prompt without reloading the application
	modelConfig, err := models.LoadAiAgentModelConfig("prompts/ai_agent_models.json", uc.config.MainAgentDefaultModel)
	if err != nil {
		return "", "", errors.Wrap(err, "could not load AI agent model configuration")
	}

	model = modelConfig.GetModelForPrompt(promptPath)

	prompt, err = preparePrompt(promptPath, data)
	if err != nil {
		return "", "", errors.Wrap(err, "could not prepare prompt")
	}

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
	if err := uc.enforceSecurityCase.ReadOrUpdateCase(c.GetMetadata(), availableInboxIds); err != nil {
		return models.Case{}, err
	}
	return c, nil
}

// Get AI setting, merge default settings with repository settings if exists
func (uc *AiAgentUsecase) getAiSetting(ctx context.Context, organizationId string) (models.AiSetting, error) {
	aiSetting := models.DefaultAiSetting()
	logger := utils.LoggerFromContext(ctx)
	logger.DebugContext(ctx, "Getting AI setting for organization", "organizationId", organizationId)
	aiSettingRepo, err := uc.repository.GetAiSetting(ctx, uc.executorFactory.NewExecutor(), organizationId)
	if err != nil {
		return models.AiSetting{}, errors.Wrap(err, "could not get ai setting")
	}
	if aiSettingRepo != nil {
		aiSetting = *aiSettingRepo
	}
	return aiSetting, nil
}
