package usecases

import (
	"archive/zip"
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/checkmarble/marble-backend/dto/agent_dto"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/checkmarble/marble-backend/usecases/inboxes"
	"github.com/checkmarble/marble-backend/usecases/security"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/pkg/errors"
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
}

type AiAgentUsecaseIngestedDataReader interface {
	ReadPivotObjectsFromValues(
		ctx context.Context,
		organizationId string,
		values []models.PivotDataWithCount,
	) ([]models.PivotObject, error)
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
	}
}

func (uc AiAgentUsecase) GetCaseDataZip(ctx context.Context, caseId string) (io.Reader, error) {
	exec := uc.executorFactory.NewExecutor()
	c, err := uc.repository.GetCaseById(ctx, exec, caseId)
	if err != nil {
		return nil, err
	}

	inboxes, err := uc.inboxReader.ListInboxes(ctx, exec, c.OrganizationId, false)
	if err != nil {
		return nil, errors.Wrap(err, "failed to list available inboxes in usecase")
	}
	availableInboxIds := make([]string, len(inboxes))
	for i, inbox := range inboxes {
		availableInboxIds[i] = inbox.Id
	}

	if err := uc.enforceSecurity.ReadOrUpdateCase(c.GetMetadata(), availableInboxIds); err != nil {
		return nil, err
	}

	tags, err := uc.repository.ListOrganizationTags(ctx, exec, c.OrganizationId, models.TagTargetCase, false)
	if err != nil {
		return nil, errors.Wrap(err, "could not retrieve tags for case")
	}
	caseEvents, err := uc.repository.ListCaseEvents(ctx, exec, caseId)
	if err != nil {
		return nil, errors.Wrap(err, "could not retrieve case events")
	}
	users, err := uc.repository.ListUsers(ctx, exec, &c.OrganizationId)
	if err != nil {
		return nil, errors.Wrap(err, "could not retrieve users for case events")
	}
	caseEventsDto := make([]agent_dto.CaseEvent, len(caseEvents))
	for i := range caseEvents {
		caseEventsDto[i] = agent_dto.AdaptCaseEventDto(caseEvents[i], users)
	}

	decisions, err := uc.repository.DecisionsByCaseId(ctx, exec, c.OrganizationId, caseId)
	if err != nil {
		return nil, errors.Wrap(err, "could not retrieve case decisions")
	}
	decisionDtos := make([]agent_dto.Decision, len(decisions))
	for i := range decisions {
		rules, err := uc.repository.ListRulesByIterationId(ctx, exec,
			decisions[i].Decision.ScenarioIterationId)
		if err != nil {
			return nil, errors.Wrapf(err, "could not retrieve rules for decision %s", decisions[i].DecisionId)
		}
		decisionDtos[i] = agent_dto.AdaptDecision(decisions[i].Decision, decisions[i].RuleExecutions, rules)
	}

	dataModel, err := uc.dataModelUsecase.GetDataModel(ctx, c.OrganizationId, models.DataModelReadOptions{
		IncludeEnums: true, IncludeNavigationOptions: true,
	})
	if err != nil {
		return nil, errors.Wrap(err, "could not retrieve data model")
	}

	pivotValues, err := uc.repository.DecisionPivotValuesByCase(ctx, exec, caseId)
	if err != nil {
		return nil, err
	}
	pivotObjects, err := uc.ingestedDataReader.ReadPivotObjectsFromValues(ctx, c.OrganizationId, pivotValues)
	if err != nil {
		return nil, errors.Wrap(err, "could not read pivot objects from values")
	}
	pivotObjectDtos, err := pure_utils.MapErr(pivotObjects, agent_dto.AdaptPivotObjectDto)
	if err != nil {
		return nil, errors.Wrap(err, "could not adapt pivot objects to DTOs")
	}

	relatedDataPerClient := make(map[string]agent_dto.CasePivotObjectData)

	for _, pivotObject := range pivotObjects {
		var pivotObjectData agent_dto.CasePivotObjectData

		previousCases, err := uc.repository.GetCasesWithPivotValue(ctx, exec,
			c.OrganizationId, pivotObject.PivotValue)
		if err != nil {
			return nil, err
		}
		relatedCases := make([]agent_dto.CaseWithDecisions, 0, 10)
		for _, previousCase := range previousCases {
			if previousCase.Id == c.Id {
				// skip the current case, we don't want to include it in the related cases
				continue
			}

			decisions, err := uc.repository.DecisionsByCaseId(ctx, exec, c.OrganizationId, previousCase.Id)
			if err != nil {
				return nil, errors.Wrapf(err, "could not retrieve decisions for previous case %s", previousCase.Id)
			}
			previousCase.Decisions = decisions
			events, err := uc.repository.ListCaseEvents(ctx, exec, previousCase.Id)
			if err != nil {
				return nil, err
			}
			previousCase.Events = events

			// don't add rules executions because we don't care for previous cases rule exec details
			relatedCases = append(relatedCases, agent_dto.AdaptCaseWithDecisionsDto(
				previousCase, tags, inboxes, nil, users))
		}
		pivotObjectData.RelatedCases = relatedCases

		relatedDataPerClient[pivotObject.PivotObjectName+"_"+pivotObject.PivotValue] = pivotObjectData
	}

	pr, pw := io.Pipe()

	writeMap := map[string]any{
		"case.json":          agent_dto.AdaptCaseDto(c, tags, inboxes, users),
		"case_events.json":   caseEventsDto,
		"decisions.json":     decisionDtos,
		"data_model.json":    agent_dto.AdaptDataModelDto(dataModel),
		"pivot_objects.json": pivotObjectDtos,
	}

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

		_, err := zipw.Create("related_data/")
		if err != nil {
			pw.CloseWithError(errors.Wrap(err, "could not create related_data/ folder in zip"))
			return
		}

		for pivotObjectStr, data := range relatedDataPerClient {
			pivotObjectFolder := fmt.Sprintf("related_data/%s/", pivotObjectStr)
			_, err := zipw.Create(pivotObjectFolder)
			if err != nil {
				pw.CloseWithError(errors.Wrapf(err, "could not create %s folder in zip", pivotObjectStr))
				return
			}

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
		}
	}()

	return pr, nil
}
