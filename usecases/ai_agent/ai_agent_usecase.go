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

var ErrAiCaseReviewNotEnabled = errors.New("AI case review is not enabled")

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
	UpdateAiCaseReviewFeedback(
		ctx context.Context,
		exec repositories.Executor,
		reviewId uuid.UUID,
		feedback models.AiCaseReviewFeedback,
	) error
	GetCaseReviewById(ctx context.Context, exec repositories.Executor, reviewId uuid.UUID) (models.AiCaseReview, error)
	GetOrganizationById(ctx context.Context, exec repositories.Executor, organizationId string) (models.Organization, error)
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
		caseId uuid.UUID,
		aiCaseReviewId uuid.UUID,
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
	caseManagerBucketUrl     string

	llmAdapter *llm_adapter.LlmAdapter
	mu         sync.Mutex
}

type sanityCheckOutput struct {
	Ok            bool   `json:"ok" jsonschema_description:"Whether the case review is ok or not" jsonschema_required:"true"`
	Justification string `json:"justification" jsonschema_description:"Detailed justification for the sanity check, only in the case of a negative answer" jsonschema_required:"false"`
}

type caseReviewOutput struct {
	CaseReview string `json:"case_review" jsonschema_description:"The case review report in markdown format"`
	Proofs     []struct {
		Id     string               `json:"id" jsonschema_description:"The ID of the object used as proof. For the organization data model, this is referred to as object_id."`
		Type   string               `json:"type" jsonschema_description:"The type of the object used as proof, could be decision or case. For the organization data model, this is referred to as trigger_object_type."`
		Origin agent_dto.OriginName `json:"origin" jsonschema_description:"The origin of the object used as proof, could be data_model or internal"`
		Reason string               `json:"reason" jsonschema_description:"The reason why this object was useful for your review"`
	} `json:"proofs" jsonschema_description:"The proofs used to generate the case review"`
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
	caseManagerBucketUrl string,
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
		caseManagerBucketUrl:     caseManagerBucketUrl,
	}
}

type customOrgInstructions struct {
	Language  *string `json:"language"`
	Structure *string `json:"structure"`
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

func (uc *AiAgentUsecase) getCaseReviewById(ctx context.Context, reviewId uuid.UUID) (agent_dto.AiCaseReviewOutputDto, error) {
	exec := uc.executorFactory.NewExecutor()
	review, err := uc.repository.GetCaseReviewById(ctx, exec, reviewId)
	if err != nil {
		return agent_dto.AiCaseReviewOutputDto{},
			errors.Wrap(err, "could not get case review by id")
	}

	blob, err := uc.blobRepository.GetBlob(ctx, review.BucketName, review.FileReference)
	if err != nil {
		return agent_dto.AiCaseReviewOutputDto{},
			errors.Wrap(err, "could not get case review file")
	}
	defer blob.ReadCloser.Close()

	var reviewDto agent_dto.AiCaseReviewDto
	if review.Status == models.AiCaseReviewStatusCompleted {
		reviewDto, err = agent_dto.UnmarshalCaseReviewDto(
			review.DtoVersion, blob.ReadCloser)
		if err != nil {
			return agent_dto.AiCaseReviewOutputDto{},
				errors.Wrap(err, "could not unmarshal case review file")
		}
	}

	var reaction *string
	if review.Reaction != nil {
		reaction = utils.Ptr(review.Reaction.String())
	}

	return agent_dto.AiCaseReviewOutputDto{
		Id:       review.Id,
		Reaction: reaction,
		Version:  review.DtoVersion,
		Review:   reviewDto,
	}, nil
}

// returns a slice of 0 or 1 case review dto, the most recent one.
func (uc *AiAgentUsecase) getMostRecentCaseReview(ctx context.Context, caseId string) ([]agent_dto.AiCaseReviewOutputDto, error) {
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

	reviewWithFeedbackDto, err := uc.getCaseReviewById(ctx, existingCaseReviewFiles[0].Id)
	if err != nil {
		return nil, errors.Wrap(err, "could not get case review by id")
	}

	return []agent_dto.AiCaseReviewOutputDto{reviewWithFeedbackDto}, nil
}

// Returns a slice of 0 or 1 case review dto, the most recent one.
func (uc *AiAgentUsecase) GetCaseReview(ctx context.Context, caseId string) ([]agent_dto.AiCaseReviewOutputDto, error) {
	_, err := uc.getCaseWithPermissions(ctx, caseId)
	if err != nil {
		return nil, err
	}

	existingReviewDtos, err := uc.getMostRecentCaseReview(ctx, caseId)
	if err != nil {
		return nil, errors.Wrap(err, "could not list case review dtos")
	}

	if len(existingReviewDtos) == 0 {
		return make([]agent_dto.AiCaseReviewOutputDto, 0), nil
	}

	return existingReviewDtos[:1], nil
}

func (uc *AiAgentUsecase) GetCaseReviewById(ctx context.Context, caseId string, reviewId uuid.UUID) (agent_dto.AiCaseReviewOutputDto, error) {
	_, err := uc.getCaseWithPermissions(ctx, caseId)
	if err != nil {
		return agent_dto.AiCaseReviewOutputDto{}, err
	}

	reviewWithFeedbackDto, err := uc.getCaseReviewById(ctx, reviewId)
	if err != nil {
		return agent_dto.AiCaseReviewOutputDto{},
			errors.Wrap(err, "could not get case review by id")
	}

	return reviewWithFeedbackDto, nil
}

// EnqueueCreateCaseReview enqueues a case review task for a given case.
// It checks if the organization has AI case review enabled and returns an ErrAiCaseReviewNotEnabled error if not.
func (uc *AiAgentUsecase) EnqueueCreateCaseReview(ctx context.Context, caseId string) error {
	c, err := uc.getCaseWithPermissions(ctx, caseId)
	if err != nil {
		return err
	}

	hasAiCaseReviewEnabled, err := uc.HasAiCaseReviewEnabled(ctx, c.OrganizationId)
	if err != nil {
		return errors.Wrap(err, "error checking if AI case review is enabled")
	}
	if !hasAiCaseReviewEnabled {
		return ErrAiCaseReviewNotEnabled
	}

	caseIdUuid, err := uuid.Parse(caseId)
	if err != nil {
		return errors.Wrap(err, "could not parse case id")
	}

	aiCaseReview := models.NewAiCaseReview(caseIdUuid, uc.caseManagerBucketUrl)
	err = uc.caseReviewFileRepository.CreateCaseReviewFile(ctx,
		uc.executorFactory.NewExecutor(),
		aiCaseReview,
	)
	if err != nil {
		return errors.Wrap(err, "Error while creating case review file")
	}

	return uc.transactionFactory.Transaction(ctx, func(tx repositories.Transaction) error {
		return uc.caseReviewTaskEnqueuer.EnqueueCaseReviewTask(ctx, tx, c.OrganizationId, caseIdUuid, aiCaseReview.Id)
	})
}

// Get the organization description from files, default to placeholder if not found
// Not all organizations have a description file
func getOrganizationDescription(ctx context.Context, organizationId string) string {
	logger := utils.LoggerFromContext(ctx)

	clientActivityDescription, err := readPrompt(fmt.Sprintf("prompts/org_desc/%s.md", organizationId))
	if err != nil {
		logger.DebugContext(ctx, "could not read organization description", "error", err)
		clientActivityDescription = "placeholder"
	}

	return clientActivityDescription
}

// Get the organization custom instructions from files, default to nil if not found
// Not all organizations have custom instructions
func getOrganizationCustomInstructions(ctx context.Context, organizationId string) customOrgInstructions {
	logger := utils.LoggerFromContext(ctx)

	file, err := os.Open(fmt.Sprintf("prompts/org_custom_instructions/%s.json", organizationId))
	if err != nil {
		logger.DebugContext(ctx, "could not open organization custom instructions file", "error", err)
		return customOrgInstructions{}
	}
	defer file.Close()

	promptBytes, err := io.ReadAll(file)
	if err != nil {
		logger.DebugContext(ctx, "could not read organization custom instructions file", "error", err)
		return customOrgInstructions{}
	}

	var result customOrgInstructions
	err = json.Unmarshal(promptBytes, &result)
	if err != nil {
		logger.DebugContext(ctx, "could not unmarshal organization custom instructions", "error", err)
		return customOrgInstructions{}
	}

	return result
}

// Return a list of instructions to give to the LLM and the model to use for the prompt
// NOTE: The model is given by `prepareRequest()`, we call it at most twice and the the last call will set the model
func (uc *AiAgentUsecase) getOrganizationInstructionsForPrompt(ctx context.Context,
	customInstructions customOrgInstructions,
) ([]string, string) {
	logger := utils.LoggerFromContext(ctx)
	instructions := []string{}
	modelToUse := ""

	if customInstructions.Language != nil {
		language, err := pure_utils.BCP47ToEnglish(*customInstructions.Language)
		if err != nil {
			logger.DebugContext(ctx, "could not convert language to english, do not format the output with language", "error", err)
		} else {
			model, customLanguagePrompt, err := uc.prepareRequest(
				"prompts/case_review/instruction_language.md",
				map[string]any{
					"language": language,
				},
			)
			if err != nil {
				logger.DebugContext(ctx, "could not read custom language prompt", "error", err)
			} else {
				instructions = append(instructions, customLanguagePrompt)
			}

			modelToUse = model
		}
	}
	if customInstructions.Structure != nil {
		model, customStructurePrompt, err := uc.prepareRequest(
			"prompts/case_review/instruction_structure.md",
			map[string]any{
				"structure": *customInstructions.Structure,
			},
		)
		if err != nil {
			logger.DebugContext(ctx, "could not read custom structure prompt", "error", err)
		} else {
			instructions = append(instructions, customStructurePrompt)
		}

		modelToUse = model
	}

	return instructions, modelToUse
}

// Contains all results from the case review process
// Update this struct during the process and expose this struct to the caller to save the results in case we need to resume it
// Didn't include the sanity check output because it's the last step and we don't need to save it
type CaseReviewContext struct {
	DataModelSummary       *string             `json:"data_model_summary"`
	FieldsToReadPerTable   map[string][]string `json:"fields_to_read_per_table"`
	RulesDefinitionsReview *string             `json:"rules_definitions_review"`
	RuleThresholds         *string             `json:"rule_thresholds"`
	CaseReview             *caseReviewOutput   `json:"case_review"`
	SanityCheck            *sanityCheckOutput  `json:"sanity_check"`
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
// 7. Format the output with the custom instructions if given (language or structure for example)
// caseReviewContext can be provided to resume the process from a previous iteration, it will be updated in place
// Depends on the context, the process can avoid calling some LLM calls
func (uc *AiAgentUsecase) CreateCaseReviewSync(
	ctx context.Context,
	caseId string,
	caseReviewContext *CaseReviewContext,
) (agent_dto.AiCaseReviewDto, error) {
	logger := utils.LoggerFromContext(ctx)

	if caseReviewContext == nil {
		caseReviewContext = &CaseReviewContext{}
	}

	client, err := uc.GetClient(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "could not create ai client")
	}

	caseData, relatedDataPerClient, err := uc.getCaseDataWithPermissions(ctx, caseId)
	if err != nil {
		return nil, errors.Wrap(err, "could not get case data")
	}

	// Get Organization prompt from file if defined, this prompt gives more details about the organization
	clientActivityDescription := getOrganizationDescription(ctx, caseData.organizationId)

	// Prepare the custom org instructions
	customOrgInstructions := getOrganizationCustomInstructions(ctx, caseData.organizationId)

	// Define the system instruction for prompt
	systemInstruction, err := readPrompt("prompts/system.md")
	if err != nil {
		logger.DebugContext(ctx, "could not read system instruction", "error", err)
		systemInstruction = "You are a compliance officer or fraud analyst. You are given a case and you need to review it step by step. Reply factually to instructions in markdown format."
	}

	if caseReviewContext.DataModelSummary == nil {
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
			WithModel(modelDataModelSummary).
			WithInstruction(systemInstruction).
			WithText(llm_adapter.RoleUser, promptDataModelSummary).
			Do(ctx, client)
		if err != nil {
			return nil, errors.Wrap(err, "could not generate data model summary")
		}

		dataModelSummary, err := requestDataModelSummary.Get(0)
		if err != nil {
			return nil, errors.Wrap(err, "could not get data model summary")
		}
		caseReviewContext.DataModelSummary = &dataModelSummary
	}

	logger.DebugContext(ctx, "================================ Data model summary ================================")
	logger.DebugContext(ctx, "Data model summary", "response", *caseReviewContext.DataModelSummary)

	// Data model object field read options
	// Here, we implicitly distinguish between "transaction" tables (based on the presence of "many" rows for a given customer)
	// and other tables (where we can afford to read all fields)
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

	if len(tablesWithLargRowNbs) > 0 {
		for customerKey, clientData := range relatedDataPerClient.Data {
			objectTables := clientData.IngestedData
			if len(objectTables) > 0 {
				// generate the map of fields to read for every table, but only once.
				if caseReviewContext.FieldsToReadPerTable == nil {
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
						WithModel(modelDataModelObjectFieldReadOptions).
						WithInstruction(systemInstruction).
						WithText(llm_adapter.RoleAi, *caseReviewContext.DataModelSummary).
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

					caseReviewContext.FieldsToReadPerTable = dataModelObjectFieldReadOptions
				}

				for tableName, fieldsToRead := range caseReviewContext.FieldsToReadPerTable {
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
	}

	if caseReviewContext.RulesDefinitionsReview == nil {
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
		caseReviewContext.RulesDefinitionsReview = &rulesDefinitionsReview
	}
	logger.DebugContext(ctx, "================================ Rules definitions review ================================")
	logger.DebugContext(ctx, "Rules definitions review", "response",
		*caseReviewContext.RulesDefinitionsReview)

	// Rule thresholds
	if caseReviewContext.RuleThresholds == nil {
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
		caseReviewContext.RuleThresholds = &ruleThresholds
	}

	logger.DebugContext(ctx, "================================ Rule thresholds ================================")
	logger.DebugContext(ctx, "Rule thresholds", "response", *caseReviewContext.RuleThresholds)

	// Finally, we can generate the case review
	if caseReviewContext.CaseReview == nil {
		modelCaseReview, promptCaseReview, err := uc.prepareRequest(
			"prompts/case_review/case_review.md",
			map[string]any{
				"case_detail":        caseData.case_,
				"case_events":        caseData.events,
				"decisions":          caseData.decisions,
				"data_model_summary": *caseReviewContext.DataModelSummary,
				"pivot_objects":      caseData.pivotData,
				"previous_cases":     relatedDataPerClient,
				"rules_summary":      *caseReviewContext.RulesDefinitionsReview,
				"rule_thresholds":    *caseReviewContext.RuleThresholds,
			},
		)
		if err != nil {
			return nil, errors.Wrap(err, "could not prepare case review request")
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
		caseReviewContext.CaseReview = &caseReview
	}
	logger.DebugContext(ctx, "================================ Full case review ================================")
	logger.DebugContext(ctx, "Full case review", "response", *caseReviewContext.CaseReview)

	if caseReviewContext.SanityCheck == nil {
		// Finally, sanity check the resulting case review using a judgement prompt
		modelSanityCheck, promptSanityCheck, err := uc.prepareRequest(
			"prompts/case_review/sanity_check.md",
			map[string]any{
				"case_detail":        caseData.case_,
				"case_events":        caseData.events,
				"decisions":          caseData.decisions,
				"data_model_summary": *caseReviewContext.DataModelSummary,
				"pivot_objects":      caseData.pivotData,
				"previous_cases":     relatedDataPerClient,
				"rules_summary":      *caseReviewContext.RulesDefinitionsReview,
				"rule_thresholds":    *caseReviewContext.RuleThresholds,
				"case_review":        *caseReviewContext.CaseReview,
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
		caseReviewContext.SanityCheck = &sanityCheck
	}

	logger.DebugContext(ctx, "================================ Sanity check ================================")
	logger.DebugContext(ctx, "Sanity check", "response", *caseReviewContext.SanityCheck)

	// Do custom language and structure instructions
	var finalOutput string = caseReviewContext.CaseReview.CaseReview

	instructions, modelForInstruction := uc.getOrganizationInstructionsForPrompt(ctx, customOrgInstructions)

	// If there is no instructions, we don't need to format the output
	if len(instructions) > 0 {
		customReportInstruction, err := readPrompt("prompts/case_review/instruction_custom_report.md")
		if err != nil {
			logger.DebugContext(ctx, "could not read custom report instruction", "error", err)
			customReportInstruction = "Transform the case review according to the instructions. Return only the transformed content without explanations or preambles."
		}

		customFormatRequest := llm_adapter.NewRequest[string]().
			WithModel(modelForInstruction).
			WithInstruction(systemInstruction).
			WithInstruction(customReportInstruction)
		// Add all custom instructions for organization
		for _, instruction := range instructions {
			logger.DebugContext(ctx, "Adding custom instruction", "instruction", instruction)
			customFormatRequest = customFormatRequest.WithInstruction(instruction)
		}
		requestCustomFormat, err := customFormatRequest.
			WithText(llm_adapter.RoleUser, finalOutput).
			Do(ctx, client)
		if err != nil {
			logger.DebugContext(ctx, "could not get custom format", "error", err)
			return nil, errors.Wrap(err, "could not get custom format")
		}
		finalOutput, err = requestCustomFormat.Get(0)
		if err != nil {
			logger.DebugContext(ctx, "could not get custom format", "error", err)
			return nil, errors.Wrap(err, "could not get custom format")
		}
	} else {
		logger.DebugContext(ctx, "No custom instructions for organization, skip this part")
	}

	logger.DebugContext(ctx, "================================ Custom format ================================")
	logger.DebugContext(ctx, "Custom format", "response", finalOutput)

	// Format the proofs
	proofs := make([]agent_dto.CaseReviewProof, len(caseReviewContext.CaseReview.Proofs))
	for i, proof := range caseReviewContext.CaseReview.Proofs {
		proofs[i] = agent_dto.CaseReviewProof{
			Id:     proof.Id,
			Type:   proof.Type,
			Origin: proof.Origin,
			Reason: proof.Reason,
		}
	}

	// Can access to Ok and Justification, the nil check is done in the sanity check step
	if caseReviewContext.SanityCheck.Ok {
		return agent_dto.CaseReviewV1{
			Ok:     caseReviewContext.SanityCheck.Ok,
			Output: finalOutput,
			Proofs: proofs,
		}, nil
	}
	return agent_dto.CaseReviewV1{
		Ok:          false,
		Output:      finalOutput,
		SanityCheck: caseReviewContext.SanityCheck.Justification,
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

func (uc *AiAgentUsecase) UpdateAiCaseReviewFeedback(
	ctx context.Context,
	caseId string,
	reviewId uuid.UUID,
	feedback models.AiCaseReviewFeedback,
) (agent_dto.AiCaseReviewOutputDto, error) {
	exec := uc.executorFactory.NewExecutor()

	_, err := uc.getCaseWithPermissions(ctx, caseId)
	if err != nil {
		return agent_dto.AiCaseReviewOutputDto{}, err
	}

	if err := uc.repository.UpdateAiCaseReviewFeedback(ctx, exec, reviewId, feedback); err != nil {
		return agent_dto.AiCaseReviewOutputDto{}, err
	}

	caseReview, err := uc.getCaseReviewById(ctx, reviewId)
	if err != nil {
		return agent_dto.AiCaseReviewOutputDto{}, err
	}

	return caseReview, nil
}

func (uc *AiAgentUsecase) HasAiCaseReviewEnabled(ctx context.Context, orgId string) (bool, error) {
	// Check if the organization has AI case review enabled, fetch the organization and check the flag
	org, err := uc.repository.GetOrganizationById(ctx, uc.executorFactory.NewExecutor(), orgId)
	if err != nil {
		return false, err
	}
	if !org.AiCaseReviewEnabled {
		return false, nil
	}
	return true, nil
}
