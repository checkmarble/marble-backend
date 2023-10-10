package usecases

import (
	"context"
	"fmt"
	"io"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/security"
	"github.com/checkmarble/marble-backend/usecases/transaction"
	"github.com/checkmarble/marble-backend/utils"
)

type ExportDecisions interface {
	ExportDecisions(scheduledExecutionId string, dest io.Writer) (int, error)
}

type ScheduledExecutionUsecaseRepository interface {
	GetScenarioById(tx repositories.Transaction, scenarioId string) (models.Scenario, error)
	GetScenarioIteration(tx repositories.Transaction, scenarioIterationId string) (
		models.ScenarioIteration, error,
	)

	GetScheduledExecution(tx repositories.Transaction, id string) (models.ScheduledExecution, error)
	ListScheduledExecutions(tx repositories.Transaction, filters models.ListScheduledExecutionsFilters) ([]models.ScheduledExecution, error)
	CreateScheduledExecution(tx repositories.Transaction, input models.CreateScheduledExecutionInput, id string) error
	UpdateScheduledExecution(tx repositories.Transaction, input models.UpdateScheduledExecutionInput) error
}

type ScheduledExecutionUsecase struct {
	enforceSecurity         security.EnforceSecurityDecision
	transactionFactory      transaction.TransactionFactory
	repository              ScheduledExecutionUsecaseRepository
	exportScheduleExecution ExportDecisions
	organizationIdOfContext func() (string, error)
}

func (usecase *ScheduledExecutionUsecase) GetScheduledExecution(id string) (models.ScheduledExecution, error) {
	return transaction.TransactionReturnValue(usecase.transactionFactory, models.DATABASE_MARBLE_SCHEMA, func(tx repositories.Transaction) (models.ScheduledExecution, error) {
		execution, err := usecase.repository.GetScheduledExecution(tx, id)
		if err != nil {
			return models.ScheduledExecution{}, err
		}
		if err := usecase.enforceSecurity.ReadScheduledExecution(execution); err != nil {
			return models.ScheduledExecution{}, err
		}
		return execution, nil
	})
}

func (usecase *ScheduledExecutionUsecase) ExportScheduledExecutionDecisions(scheduledExecutionID string, w io.Writer) (int, error) {
	return transaction.TransactionReturnValue(usecase.transactionFactory, models.DATABASE_MARBLE_SCHEMA, func(tx repositories.Transaction) (int, error) {
		execution, err := usecase.repository.GetScheduledExecution(tx, scheduledExecutionID)
		if err != nil {
			return 0, err
		}
		if err := usecase.enforceSecurity.ReadScheduledExecution(execution); err != nil {
			return 0, err
		}

		return usecase.exportScheduleExecution.ExportDecisions(execution.Id, w)
	})
}

// ListScheduledExecutions returns the list of scheduled executions of the current organization.
// The optional argument 'scenarioId' can be used to filter the returned list.
func (usecase *ScheduledExecutionUsecase) ListScheduledExecutions(scenarioId string) ([]models.ScheduledExecution, error) {

	return transaction.TransactionReturnValue(usecase.transactionFactory, models.DATABASE_MARBLE_SCHEMA, func(tx repositories.Transaction) ([]models.ScheduledExecution, error) {

		var executions []models.ScheduledExecution
		if scenarioId == "" {
			organizationId, err := usecase.organizationIdOfContext()
			if err != nil {
				return nil, err
			}

			executions, err = usecase.repository.ListScheduledExecutions(tx, models.ListScheduledExecutionsFilters{OrganizationId: organizationId})
			if err != nil {
				return []models.ScheduledExecution{}, err
			}
		} else {
			var err error
			executions, err = usecase.repository.ListScheduledExecutions(tx, models.ListScheduledExecutionsFilters{ScenarioId: scenarioId})
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

func (usecase *ScheduledExecutionUsecase) CreateScheduledExecution(input models.CreateScheduledExecutionInput) error {
	if err := usecase.enforceSecurity.CreateScheduledExecution(input.OrganizationId); err != nil {
		return err
	}

	scenarioIteration, err := usecase.repository.GetScenarioIteration(nil, input.ScenarioIterationId)
	if err != nil {
		return err
	}
	scenario, err := usecase.repository.GetScenarioById(nil, scenarioIteration.ScenarioId)
	if err != nil {
		return err
	}

	if *scenario.LiveVersionID != scenarioIteration.Id {
		return fmt.Errorf("scenario iteration is not live %w", models.BadParameterError)
	}

	pendingExecutions, err := usecase.repository.ListScheduledExecutions(nil, models.ListScheduledExecutionsFilters{ScenarioId: input.ScenarioId, Status: []models.ScheduledExecutionStatus{models.ScheduledExecutionPending, models.ScheduledExecutionProcessing}})
	if err != nil {
		return err
	}
	if len(pendingExecutions) > 0 {
		return fmt.Errorf("A pending execution already exists for this scenario %w", models.BadParameterError)
	}

	id := utils.NewPrimaryKey(input.OrganizationId)
	return usecase.transactionFactory.Transaction(models.DATABASE_MARBLE_SCHEMA, func(tx repositories.Transaction) error {
		return usecase.repository.CreateScheduledExecution(tx, models.CreateScheduledExecutionInput{
			OrganizationId:      input.OrganizationId,
			ScenarioId:          scenario.Id,
			ScenarioIterationId: input.ScenarioIterationId,
			Manual:              true,
		}, id)
	})
}

func (usecase *ScheduledExecutionUsecase) UpdateScheduledExecution(ctx context.Context, input models.UpdateScheduledExecutionInput) error {

	return usecase.transactionFactory.Transaction(models.DATABASE_MARBLE_SCHEMA, func(tx repositories.Transaction) error {
		execution, err := usecase.repository.GetScheduledExecution(tx, input.Id)
		if err != nil {
			return err
		}
		if err := usecase.enforceSecurity.CreateScheduledExecution(execution.OrganizationId); err != nil {
			return err
		}
		return usecase.repository.UpdateScheduledExecution(tx, input)
	})
}
