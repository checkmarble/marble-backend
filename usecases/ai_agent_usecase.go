package usecases

import (
	"archive/zip"
	"context"
	"encoding/json"
	"io"

	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/dto/agent_dto"
	"github.com/checkmarble/marble-backend/models"
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
	GetRuleById(ctx context.Context, exec repositories.Executor, ruleId string) (models.Rule, error)
	ListUsers(ctx context.Context, exec repositories.Executor, organizationIDFilter *string) ([]models.User, error)
	DecisionsByCaseId(ctx context.Context, exec repositories.Executor, orgId string, caseId string) (
		[]models.DecisionWithRuleExecutions, error)
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

	availableInboxIds, err := uc.getAvailableInboxIds(ctx, exec, c.OrganizationId)
	if err != nil {
		return nil, err
	}
	if err := uc.enforceSecurity.ReadOrUpdateCase(c.GetMetadata(), availableInboxIds); err != nil {
		return nil, err
	}

	caseDto := dto.AdaptCaseDto(c)
	caseJson, err := json.Marshal(caseDto)
	if err != nil {
		return nil, errors.Wrap(err, "could not marshal case to JSON")
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
	caseEventsJson, err := json.Marshal(caseEventsDto)
	if err != nil {
		return nil, errors.Wrap(err, "could not marshal case events to JSON")
	}

	decisions, err := uc.repository.DecisionsByCaseId(ctx, exec, c.OrganizationId, caseId)
	if err != nil {
		return nil, errors.Wrap(err, "could not retrieve case decisions")
	}
	decisionDtos := make([]agent_dto.Decision, len(decisions))
	for i := range decisions {
		rulesWithDetails := make([]agent_dto.DecisionRule, len(decisions[i].RuleExecutions))
		for j, decRule := range decisions[i].RuleExecutions {
			rule, err := uc.repository.GetRuleById(ctx, exec, decRule.Rule.Id)
			if err != nil {
				return nil, errors.Wrapf(err, "could not retrieve rule %s for decision %s", decRule.Id, decisions[i].DecisionId)
			}
			rulesWithDetails[j] = agent_dto.AcaptDecisionRule(decRule, rule)
		}
		decisionDtos[i] = agent_dto.AdaptDecision(decisions[i].Decision, rulesWithDetails)
	}
	decisionsJson, err := json.Marshal(decisionDtos)
	if err != nil {
		return nil, errors.Wrap(err, "could not marshal case decisions to JSON")
	}

	dataModel, err := uc.dataModelUsecase.GetDataModel(ctx, c.OrganizationId, models.DataModelReadOptions{
		IncludeEnums: true, IncludeNavigationOptions: true,
	})
	if err != nil {
		return nil, errors.Wrap(err, "could not retrieve data model")
	}
	dataModelDto := agent_dto.AdaptDataModelDto(dataModel)
	dataModelJson, err := json.Marshal(dataModelDto)
	if err != nil {
		return nil, errors.Wrap(err, "could not marshal data model to JSON")
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

		// Write case.json file
		f, err := zipw.Create("case.json")
		if err != nil {
			pw.CloseWithError(errors.Wrap(err, "could not create case.json in zip"))
			return
		}
		if _, err := f.Write(caseJson); err != nil {
			pw.CloseWithError(errors.Wrap(err, "could not write case JSON to zip"))
			return
		}

		// write case events file
		f, err = zipw.Create("case_events.json")
		if err != nil {
			pw.CloseWithError(errors.Wrap(err, "could not create case_events.json in zip"))
			return
		}
		if _, err = f.Write(caseEventsJson); err != nil {
			pw.CloseWithError(errors.Wrap(err, "could not write case events JSON to zip"))
			return
		}

		// write decisions file
		f, err = zipw.Create("decisions.json")
		if err != nil {
			pw.CloseWithError(errors.Wrap(err, "could not create decisions.json in zip"))
			return
		}
		if _, err = f.Write(decisionsJson); err != nil {
			pw.CloseWithError(errors.Wrap(err, "could not write decisions JSON to zip"))
			return
		}

		// write data model file
		f, err = zipw.Create("data_model.json")
		if err != nil {
			pw.CloseWithError(errors.Wrap(err, "could not create data_model.json in zip"))
			return
		}
		if _, err = f.Write(dataModelJson); err != nil {
			pw.CloseWithError(errors.Wrap(err, "could not write data model JSON to zip"))
			return
		}

		// Add more files below by doing more calls to zipw.Create() and writing to the returned io.Writer
	}()

	return pr, nil
}

func (usecase AiAgentUsecase) getAvailableInboxIds(ctx context.Context, exec repositories.Executor, organizationId string) ([]string, error) {
	inboxes, err := usecase.inboxReader.ListInboxes(ctx, exec, organizationId, false)
	if err != nil {
		return []string{}, errors.Wrap(err, "failed to list available inboxes in usecase")
	}
	availableInboxIds := make([]string, len(inboxes))
	for i, inbox := range inboxes {
		availableInboxIds[i] = inbox.Id
	}
	return availableInboxIds, nil
}
