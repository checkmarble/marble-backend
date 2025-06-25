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
		gcpRegion:          utils.GetEnv("GCP_REGION", "europe-west1"),
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
	caseDtos, relatedDataPerClient, err := uc.getCaseData(ctx, caseId)
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

		for pivotObjectStr, data := range relatedDataPerClient {
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
				if len(objects) == 0 {
					continue
				}
				fileStr := pivotObjectFolder + tableName + ".csv"
				f, err := zipw.Create(fileStr)
				if err != nil {
					pw.CloseWithError(errors.Wrapf(err, "could not create %s in zip", fileStr))
					return
				}
				csvFile := csv.NewWriter(f)
				if err := agent_dto.WriteClientDataToCsv(objects, csvFile); err != nil {
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

func (uc *AiAgentUsecase) generateContent(
	ctx context.Context,
	client *genai.Client,
	promptPath string,
	data map[string]any,
	tools []*genai.Tool,
) (string, error) {
	prompt, err := readPrompt(promptPath)
	if err != nil {
		return "", err
	}

	// Load model configuration on each call
	modelConfig, err := models.LoadAiAgentModelConfig("prompts/ai_agent_models.json")
	if err != nil {
		return "", errors.Wrap(err, "could not load AI agent model configuration")
	}

	// Get the appropriate model for this prompt
	model := modelConfig.GetModelForPrompt(promptPath)

	logger := utils.LoggerFromContext(ctx)
	logger.InfoContext(ctx, "using model for prompt",
		"prompt", promptPath,
		"model", model)

	marshalledMap := make(map[string]string)
	for k, v := range data {
		b, err := json.Marshal(v)
		if err != nil {
			return "", errors.Wrapf(err, "could not marshal %s", k)
		}
		marshalledMap[k] = string(b)
	}

	t := template.Must(template.New(promptPath).Parse(prompt))
	buf := bytes.Buffer{}
	err = t.Execute(&buf, marshalledMap)
	if err != nil {
		return "", errors.Wrap(err, "could not execute template")
	}
	prompt = buf.String()
	result, err := client.Models.GenerateContent(ctx,
		model,
		genai.Text(prompt),
		&genai.GenerateContentConfig{
			Tools: tools,
		},
	)
	if err != nil {
		return "", err
	}
	if len(result.Candidates) == 0 {
		return "", errors.New("no response from GenAI")
	}
	onlyTextParts := make([]string, 0, len(result.Candidates[0].Content.Parts))
	for _, part := range result.Candidates[0].Content.Parts {
		if part.Text != "" {
			onlyTextParts = append(onlyTextParts, part.Text)
		}
	}
	logger.InfoContext(ctx, "content detail",
		"prompt", promptPath,
		"model", model,
		"len", len(result.Candidates[0].Content.Parts),
		"len_filtered_text", len(onlyTextParts),
	)
	gatherText := ""
	for _, t := range onlyTextParts {
		gatherText += t
	}
	return gatherText, nil
}

func (uc *AiAgentUsecase) CreateCaseReview(ctx context.Context, caseId string) (string, error) {
	client, err := uc.GetClient(ctx)
	if err != nil {
		return "", errors.Wrap(err, "could not create GenAI client")
	}

	caseDtos, relatedDataPerClient, err := uc.getCaseData(ctx, caseId)
	if err != nil {
		return "", errors.Wrap(err, "could not get case data")
	}

	var clientActivityDescription string
	clientActivityDescription, err = readPrompt(fmt.Sprintf("prompts/org_desc/%s.md", caseDtos.organizationId))
	if err != nil {
		logger := utils.LoggerFromContext(ctx)
		logger.ErrorContext(ctx, "could not read organization description", "error", err)
		clientActivityDescription = "placeholder"
	}

	dataModelSummary, err := uc.generateContent(ctx,
		client,
		"prompts/case_review/data_model_summary.md",
		map[string]any{
			"data_model": caseDtos.dataModel,
		},
		nil,
	)
	if err != nil {
		return "", errors.Wrap(err, "could not generate data model summary")
	}
	fmt.Println("-------------------------------- Data model summary --------------------------------")
	fmt.Println("Data Model Summary:", dataModelSummary)

	rulesDefinitionsReview, err := uc.generateContent(ctx,
		client,
		"prompts/case_review/rule_definitions.md",
		map[string]any{
			"decisions":            caseDtos.decisions,
			"activity_description": clientActivityDescription,
		},
		[]*genai.Tool{
			{GoogleSearch: &genai.GoogleSearch{}},
		},
	)
	if err != nil {
		return "", errors.Wrap(err, "could not generate rules definitions review")
	}
	fmt.Println("-------------------------------- Rules definitions review --------------------------------")
	fmt.Println("Rules Review:", rulesDefinitionsReview)

	ruleThresholds, err := uc.generateContent(ctx,
		client,
		"prompts/case_review/rule_threshold_values.md",
		map[string]any{
			"decisions": caseDtos.decisions,
		},
		nil,
	)
	if err != nil {
		return "", errors.Wrap(err, "could not generate rule thresholds")
	}
	fmt.Println("-------------------------------- Rule thresholds --------------------------------")
	fmt.Println("Rule Thresholds:", ruleThresholds)

	// Finally, we can generate the case review
	caseReview, err := uc.generateContent(
		ctx,
		client,
		"prompts/case_review/case_review.md",
		map[string]any{
			"case_detail":        caseDtos.case_,
			"case_events":        caseDtos.events,
			"decisions":          caseDtos.decisions,
			"data_model_summary": dataModelSummary,
			"pivot_objects":      caseDtos.pivotData,
			"previous_cases":     relatedDataPerClient,
			"rules_summary":      rulesDefinitionsReview,
			"rule_thresholds":    ruleThresholds,
		},
		[]*genai.Tool{
			{GoogleSearch: &genai.GoogleSearch{}},
		},
	)
	if err != nil {
		return "", errors.Wrap(err, "could not generate case review")
	}
	fmt.Println("-------------------------------- Full Case Review --------------------------------")
	fmt.Println("Case Review:", caseReview)

	// Finally, sanity check the resulting case review using a judgement prompt
	sanityCheck, err := uc.generateContent(ctx,
		client,
		"prompts/case_review/sanity_check.md",
		map[string]any{
			"case_detail":        caseDtos.case_,
			"case_events":        caseDtos.events,
			"decisions":          caseDtos.decisions,
			"data_model_summary": dataModelSummary,
			"pivot_objects":      caseDtos.pivotData,
			"previous_cases":     relatedDataPerClient,
			"rules_summary":      rulesDefinitionsReview,
			"rule_thresholds":    ruleThresholds,
			"case_review":        caseReview,
		},
		nil,
	)
	if err != nil {
		return "", errors.Wrap(err, "could not generate sanity check")
	}
	fmt.Println("-------------------------------- Sanity Check --------------------------------")
	fmt.Println("Sanity Check:", sanityCheck)

	if len(sanityCheck) > 1 && sanityCheck[:2] == "ok" {
		return caseReview, nil
	}
	return fmt.Sprintf("Review is ko: original review:%s\nsanity check output:%s", caseReview, sanityCheck), nil
}

func (uc *AiAgentUsecase) getCaseData(ctx context.Context, caseId string) (caseData, map[string]agent_dto.CasePivotObjectData, error) {
	exec := uc.executorFactory.NewExecutor()
	c, err := uc.repository.GetCaseById(ctx, exec, caseId)
	if err != nil {
		return caseData{}, nil, err
	}

	inboxes, err := uc.inboxReader.ListInboxes(ctx, exec, c.OrganizationId, false)
	if err != nil {
		return caseData{}, nil, errors.Wrap(err, "failed to list available inboxes in usecase")
	}
	availableInboxIds := make([]uuid.UUID, len(inboxes))
	for i, inbox := range inboxes {
		availableInboxIds[i] = inbox.Id
	}

	if err := uc.enforceSecurity.ReadOrUpdateCase(c.GetMetadata(), availableInboxIds); err != nil {
		return caseData{}, nil, err
	}

	tags, err := uc.repository.ListOrganizationTags(ctx, exec, c.OrganizationId, models.TagTargetCase, false)
	if err != nil {
		return caseData{}, nil, errors.Wrap(err, "could not retrieve tags for case")
	}
	caseEvents, err := uc.repository.ListCaseEvents(ctx, exec, caseId)
	if err != nil {
		return caseData{}, nil, errors.Wrap(err, "could not retrieve case events")
	}
	users, err := uc.repository.ListUsers(ctx, exec, &c.OrganizationId)
	if err != nil {
		return caseData{}, nil, errors.Wrap(err, "could not retrieve users for case events")
	}
	caseEventsDto := make([]agent_dto.CaseEvent, len(caseEvents))
	for i := range caseEvents {
		caseEventsDto[i] = agent_dto.AdaptCaseEventDto(caseEvents[i], users)
	}

	decisions, err := uc.repository.DecisionsByCaseId(ctx, exec, c.OrganizationId, caseId)
	if err != nil {
		return caseData{}, nil, errors.Wrap(err, "could not retrieve case decisions")
	}
	decisionDtos := make([]agent_dto.Decision, len(decisions))
	for i := range decisions {
		iteration, err := uc.repository.GetScenarioIteration(ctx, exec,
			decisions[i].Decision.ScenarioIterationId)
		if err != nil {
			return caseData{}, nil, errors.Wrapf(err,
				"could not retrieve scenario for decision %s", decisions[i].DecisionId)
		}
		rules, err := uc.repository.ListRulesByIterationId(ctx, exec,
			decisions[i].Decision.ScenarioIterationId)
		if err != nil {
			return caseData{}, nil, errors.Wrapf(err,
				"could not retrieve rules for decision %s", decisions[i].DecisionId)
		}
		screenings, err := uc.repository.ListScreeningsForDecision(ctx, exec, decisions[i].DecisionId, true)
		if err != nil {
			return caseData{}, nil, errors.Wrapf(err,
				"could not retrieve screenings for decision %s", decisions[i].DecisionId)
		}
		decisionDtos[i] = agent_dto.AdaptDecision(decisions[i].Decision, iteration,
			decisions[i].RuleExecutions, rules, screenings)
	}

	dataModel, err := uc.dataModelUsecase.GetDataModel(ctx, c.OrganizationId, models.DataModelReadOptions{
		IncludeEnums: true, IncludeNavigationOptions: true,
	})
	if err != nil {
		return caseData{}, nil, errors.Wrap(err, "could not retrieve data model")
	}

	pivotValues, err := uc.repository.DecisionPivotValuesByCase(ctx, exec, caseId)
	if err != nil {
		return caseData{}, nil, err
	}
	pivotObjects, err := uc.ingestedDataReader.ReadPivotObjectsFromValues(ctx, c.OrganizationId, pivotValues)
	if err != nil {
		return caseData{}, nil, errors.Wrap(err, "could not read pivot objects from values")
	}
	pivotObjectDtos, err := pure_utils.MapErr(pivotObjects, agent_dto.AdaptPivotObjectDto)
	if err != nil {
		return caseData{}, nil, errors.Wrap(err, "could not adapt pivot objects to DTOs")
	}

	relatedDataPerClient := make(map[string]agent_dto.CasePivotObjectData)

	for _, pivotObject := range pivotObjects {
		pivotObjectData := agent_dto.CasePivotObjectData{
			IngestedData: make(map[string][]models.ClientObjectDetail, 10),
			RelatedCases: make([]agent_dto.CaseWithDecisions, 0, 10),
		}

		previousCases, err := uc.repository.GetCasesWithPivotValue(ctx, exec,
			c.OrganizationId, pivotObject.PivotValue)
		if err != nil {
			return caseData{}, nil, err
		}
		relatedCases := make([]agent_dto.CaseWithDecisions, 0, 10)
		for _, previousCase := range previousCases {
			if previousCase.Id == c.Id {
				// skip the current case, we don't want to include it in the related cases
				continue
			}

			decisions, err := uc.repository.DecisionsByCaseId(ctx, exec, c.OrganizationId, previousCase.Id)
			if err != nil {
				return caseData{}, nil, errors.Wrapf(err,
					"could not retrieve decisions for previous case %s", previousCase.Id)
			}
			previousCase.Decisions = decisions
			events, err := uc.repository.ListCaseEvents(ctx, exec, previousCase.Id)
			if err != nil {
				return caseData{}, nil, err
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
				return caseData{}, nil, errors.Wrapf(err,
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
				objects, _, _, err := uc.ingestedDataReader.ReadIngestedClientObjects(ctx,
					c.OrganizationId, navOption.TargetTableName, models.ClientDataListRequestBody{
						ExplorationOptions: models.ExplorationOptions{
							SourceTableName:   pivotObject.PivotObjectName,
							FilterFieldName:   navOption.FilterFieldName,
							FilterFieldValue:  models.NewStringOrNumberFromString(sourceFieldValueStr),
							OrderingFieldName: navOption.OrderingFieldName,
						},
						Limit: 1000,
					})
				if err != nil {
					return caseData{}, nil, errors.Wrapf(err,
						"could not read ingested client objects for %s with value %s",
						pivotObject.PivotObjectName, sourceFieldValueStr)
				}
				pivotObjectData.IngestedData[navOption.TargetTableName] = objects
			}
		}

		relatedDataPerClient[pivotObject.PivotObjectName+"_"+pivotObject.PivotValue] = pivotObjectData
	}

	return caseData{
		case_:          agent_dto.AdaptCaseDto(c, tags, inboxes, users),
		events:         caseEventsDto,
		decisions:      decisionDtos,
		dataModel:      agent_dto.AdaptDataModelDto(dataModel),
		pivotData:      pivotObjectDtos,
		organizationId: c.OrganizationId,
	}, relatedDataPerClient, nil
}

type caseData struct {
	case_          agent_dto.Case
	events         []agent_dto.CaseEvent
	decisions      []agent_dto.Decision
	dataModel      agent_dto.DataModel
	pivotData      []agent_dto.PivotObject
	organizationId string
}
