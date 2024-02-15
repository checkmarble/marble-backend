package usecases

import (
	"context"
	"fmt"
	"io"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/checkmarble/marble-backend/usecases/security"
	"github.com/checkmarble/marble-backend/utils"
)

type ExportDecisions interface {
	ExportDecisions(ctx context.Context, scheduledExecutionId string, dest io.Writer) (int, error)
}

type ScheduledExecutionUsecaseRepository interface {
	GetScenarioById(ctx context.Context, exec repositories.Executor, scenarioId string) (models.Scenario, error)
	GetScenarioIteration(ctx context.Context, exec repositories.Executor, scenarioIterationId string) (
		models.ScenarioIteration, error,
	)

	GetScheduledExecution(ctx context.Context, exec repositories.Executor, id string) (models.ScheduledExecution, error)
	ListScheduledExecutions(ctx context.Context, exec repositories.Executor, filters models.ListScheduledExecutionsFilters) ([]models.ScheduledExecution, error)
	CreateScheduledExecution(ctx context.Context, exec repositories.Executor, input models.CreateScheduledExecutionInput, id string) error
	UpdateScheduledExecution(ctx context.Context, exec repositories.Executor, input models.UpdateScheduledExecutionInput) error
}

type ScheduledExecutionUsecase struct {
	enforceSecurity         security.EnforceSecurityDecision
	transactionFactory      executor_factory.TransactionFactory
	executorFactory         executor_factory.ExecutorFactory
	repository              ScheduledExecutionUsecaseRepository
	exportScheduleExecution ExportDecisions
	organizationIdOfContext func() (string, error)
}

func (usecase *ScheduledExecutionUsecase) GetScheduledExecution(ctx context.Context, id string) (models.ScheduledExecution, error) {
	return executor_factory.TransactionReturnValue(ctx, usecase.transactionFactory, func(tx repositories.Executor) (models.ScheduledExecution, error) {
		execution, err := usecase.repository.GetScheduledExecution(ctx, tx, id)
		if err != nil {
			return models.ScheduledExecution{}, err
		}
		if err := usecase.enforceSecurity.ReadScheduledExecution(execution); err != nil {
			return models.ScheduledExecution{}, err
		}
		return execution, nil
	})
}

func (usecase *ScheduledExecutionUsecase) ExportScheduledExecutionDecisions(ctx context.Context, scheduledExecutionID string, w io.Writer) (int, error) {
	return executor_factory.TransactionReturnValue(ctx, usecase.transactionFactory, func(tx repositories.Executor) (int, error) {
		execution, err := usecase.repository.GetScheduledExecution(ctx, tx, scheduledExecutionID)
		if err != nil {
			return 0, err
		}
		if err := usecase.enforceSecurity.ReadScheduledExecution(execution); err != nil {
			return 0, err
		}

		return usecase.exportScheduleExecution.ExportDecisions(ctx, execution.Id, w)
	})
}

// ListScheduledExecutions returns the list of scheduled executions of the current organization.
// The optional argument 'scenarioId' can be used to filter the returned list.
func (usecase *ScheduledExecutionUsecase) ListScheduledExecutions(ctx context.Context, scenarioId string) ([]models.ScheduledExecution, error) {
	return executor_factory.TransactionReturnValue(ctx, usecase.transactionFactory, func(tx repositories.Executor) ([]models.ScheduledExecution, error) {
		var executions []models.ScheduledExecution
		if scenarioId == "" {
			organizationId, err := usecase.organizationIdOfContext()
			if err != nil {
				return nil, err
			}

			executions, err = usecase.repository.ListScheduledExecutions(ctx, tx, models.ListScheduledExecutionsFilters{OrganizationId: organizationId})
			if err != nil {
				return []models.ScheduledExecution{}, err
			}
		} else {
			var err error
			executions, err = usecase.repository.ListScheduledExecutions(ctx, tx, models.ListScheduledExecutionsFilters{ScenarioId: scenarioId})
			if err != nil {
				return []models.ScheduledExecution{}, err
			}
		}

		// security check
		for _, execution := range executions {
			if err := usecase.enforceSecurity.ReadScheduledExecution(execution); err != nil {
				return []models.ScheduledExecution{}, err
			}
		}
		return executions, nil
	})
}

func (usecase *ScheduledExecutionUsecase) CreateScheduledExecution(ctx context.Context, input models.CreateScheduledExecutionInput) error {
	exec := usecase.executorFactory.NewExecutor()
	if err := usecase.enforceSecurity.CreateScheduledExecution(input.OrganizationId); err != nil {
		return err
	}

	scenarioIteration, err := usecase.repository.GetScenarioIteration(ctx, exec, input.ScenarioIterationId)
	if err != nil {
		return err
	}
	scenario, err := usecase.repository.GetScenarioById(ctx, exec, scenarioIteration.ScenarioId)
	if err != nil {
		return err
	}

	if *scenario.LiveVersionID != scenarioIteration.Id {
		return fmt.Errorf("scenario iteration is not live %w", models.BadParameterError)
	}

	pendingExecutions, err := usecase.repository.ListScheduledExecutions(ctx, exec, models.ListScheduledExecutionsFilters{ScenarioId: scenario.Id, Status: []models.ScheduledExecutionStatus{models.ScheduledExecutionPending, models.ScheduledExecutionProcessing}})
	if err != nil {
		return err
	}
	if len(pendingExecutions) > 0 {
		return fmt.Errorf("a pending execution already exists for this scenario %w", models.BadParameterError)
	}

	id := utils.NewPrimaryKey(input.OrganizationId)
	return usecase.transactionFactory.Transaction(ctx, func(tx repositories.Executor) error {
		return usecase.repository.CreateScheduledExecution(ctx, tx, models.CreateScheduledExecutionInput{
			OrganizationId:      input.OrganizationId,
			ScenarioId:          scenario.Id,
			ScenarioIterationId: input.ScenarioIterationId,
			Manual:              true,
		}, id)
	})
}

func (usecase *ScheduledExecutionUsecase) UpdateScheduledExecution(ctx context.Context, input models.UpdateScheduledExecutionInput) error {
	return usecase.transactionFactory.Transaction(ctx, func(tx repositories.Executor) error {
		execution, err := usecase.repository.GetScheduledExecution(ctx, tx, input.Id)
		if err != nil {
			return err
		}
		if err := usecase.enforceSecurity.CreateScheduledExecution(execution.OrganizationId); err != nil {
			return err
		}
		return usecase.repository.UpdateScheduledExecution(ctx, tx, input)
	})
}
