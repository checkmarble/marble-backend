package usecases

import (
	"context"
	"fmt"
	"io"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/checkmarble/marble-backend/usecases/security"
)

type ExportDecisions interface {
	ExportDecisions(ctx context.Context, organizationId string, scheduledExecutionId string, dest io.Writer) (int, error)
}

type ScheduledExecutionUsecaseRepository interface {
	GetScenarioById(ctx context.Context, exec repositories.Executor, scenarioId string) (models.Scenario, error)
	GetScenarioIteration(ctx context.Context, exec repositories.Executor, scenarioIterationId string) (
		models.ScenarioIteration, error,
	)

	GetScheduledExecution(ctx context.Context, exec repositories.Executor, id string) (models.ScheduledExecution, error)
	ListScheduledExecutions(ctx context.Context, exec repositories.Executor,
		filters models.ListScheduledExecutionsFilters) ([]models.ScheduledExecution, error)
	ListPaginatedScheduledExecutions(ctx context.Context, exec repositories.Executor,
		filters models.ListScheduledExecutionsFilters, paging models.PaginationAndSorting) ([]models.ScheduledExecution, error)
	CreateScheduledExecution(ctx context.Context, exec repositories.Executor,
		input models.CreateScheduledExecutionInput, id string) error
	UpdateScheduledExecutionStatus(
		ctx context.Context,
		exec repositories.Executor,
		input models.UpdateScheduledExecutionStatusInput,
	) (executed bool, err error)
}

type ScheduledExecutionUsecase struct {
	enforceSecurity         security.EnforceSecurityDecision
	transactionFactory      executor_factory.TransactionFactory
	executorFactory         executor_factory.ExecutorFactory
	repository              ScheduledExecutionUsecaseRepository
	exportScheduleExecution ExportDecisions
}

func (usecase *ScheduledExecutionUsecase) GetScheduledExecution(ctx context.Context, id string) (models.ScheduledExecution, error) {
	return executor_factory.TransactionReturnValue(ctx, usecase.transactionFactory, func(
		tx repositories.Transaction,
	) (models.ScheduledExecution, error) {
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

func (usecase *ScheduledExecutionUsecase) ExportScheduledExecutionDecisions(
	ctx context.Context,
	organizationId string,
	scheduledExecutionID string,
	w io.Writer,
) (int, error) {
	return executor_factory.TransactionReturnValue(ctx, usecase.transactionFactory, func(
		tx repositories.Transaction,
	) (int, error) {
		execution, err := usecase.repository.GetScheduledExecution(ctx, tx, scheduledExecutionID)
		if err != nil {
			return 0, err
		}
		if err := usecase.enforceSecurity.ReadScheduledExecution(execution); err != nil {
			return 0, err
		}

		return usecase.exportScheduleExecution.ExportDecisions(ctx, organizationId, execution.Id, w)
	})
}

// ListScheduledExecutions returns the list of scheduled executions of the current organization.
// The optional argument 'scenarioId' can be used to filter the returned list.
func (usecase *ScheduledExecutionUsecase) ListScheduledExecutions(
	ctx context.Context,
	organizationId string,
	scenarioId *string,
) ([]models.ScheduledExecution, error) {
	return executor_factory.TransactionReturnValue(ctx, usecase.transactionFactory, func(
		tx repositories.Transaction,
	) ([]models.ScheduledExecution, error) {
		var executions []models.ScheduledExecution
		var err error
		if scenarioId == nil {
			executions, err = usecase.repository.ListScheduledExecutions(ctx, tx, models.ListScheduledExecutionsFilters{
				OrganizationId: organizationId,
			})
			if err != nil {
				return []models.ScheduledExecution{}, err
			}
		} else {
			var err error
			executions, err = usecase.repository.ListScheduledExecutions(ctx, tx, models.ListScheduledExecutionsFilters{
				ScenarioId: *scenarioId,
			})
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

func (usecase *ScheduledExecutionUsecase) ListPaginatedScheduledExecutions(
	ctx context.Context,
	organizationId string,
	filters models.ListScheduledExecutionsFilters,
	paging models.PaginationAndSorting,
) (models.PaginatedScheduledExecutions, error) {
	return executor_factory.TransactionReturnValue(ctx, usecase.transactionFactory, func(
		tx repositories.Transaction,
	) (models.PaginatedScheduledExecutions, error) {
		executions, err := usecase.repository.ListPaginatedScheduledExecutions(ctx, tx, filters, paging)
		if err != nil {
			return models.PaginatedScheduledExecutions{}, err
		}

		for _, execution := range executions {
			if err := usecase.enforceSecurity.ReadScheduledExecution(execution); err != nil {
				return models.PaginatedScheduledExecutions{}, err
			}
		}

		hasMore := false

		if len(executions) > paging.Limit {
			hasMore = true
			executions = executions[:paging.Limit]
		}

		return models.PaginatedScheduledExecutions{Executions: executions, HasMore: hasMore}, nil
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

	pendingExecutions, err := usecase.repository.ListScheduledExecutions(ctx, exec, models.ListScheduledExecutionsFilters{
		ScenarioId: scenario.Id, Status: []models.ScheduledExecutionStatus{
			models.ScheduledExecutionPending, models.ScheduledExecutionProcessing,
		},
	})
	if err != nil {
		return err
	}
	if len(pendingExecutions) > 0 {
		return fmt.Errorf("a pending execution already exists for this scenario %w", models.BadParameterError)
	}

	id := pure_utils.NewPrimaryKey(input.OrganizationId)
	return usecase.repository.CreateScheduledExecution(ctx, exec, models.CreateScheduledExecutionInput{
		OrganizationId:      input.OrganizationId,
		ScenarioId:          scenario.Id,
		ScenarioIterationId: input.ScenarioIterationId,
		Manual:              true,
	}, id)
}
