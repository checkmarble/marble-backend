package usecases

import (
	"context"
	"io"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/scheduledexecution"
	"github.com/checkmarble/marble-backend/usecases/security"
	"github.com/checkmarble/marble-backend/utils"
)

type ScheduledExecutionUsecase struct {
	enforceSecurity              security.EnforceSecurityDecision
	transactionFactory           repositories.TransactionFactory
	scheduledExecutionRepository repositories.ScheduledExecutionRepository
	exportScheduleExecution      scheduledexecution.ExportScheduleExecution
	organizationIdOfContext      func() (string, error)
}

func (usecase *ScheduledExecutionUsecase) GetScheduledExecution(id string) (models.ScheduledExecution, error) {
	return repositories.TransactionReturnValue(usecase.transactionFactory, models.DATABASE_MARBLE_SCHEMA, func(tx repositories.Transaction) (models.ScheduledExecution, error) {
		execution, err := usecase.scheduledExecutionRepository.GetScheduledExecution(tx, id)
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
	return repositories.TransactionReturnValue(usecase.transactionFactory, models.DATABASE_MARBLE_SCHEMA, func(tx repositories.Transaction) (int, error) {
		execution, err := usecase.scheduledExecutionRepository.GetScheduledExecution(tx, scheduledExecutionID)
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

	return repositories.TransactionReturnValue(usecase.transactionFactory, models.DATABASE_MARBLE_SCHEMA, func(tx repositories.Transaction) ([]models.ScheduledExecution, error) {

		var executions []models.ScheduledExecution
		if scenarioId == "" {
			organizationId, err := usecase.organizationIdOfContext()
			if err != nil {
				return nil, err
			}

			executions, err = usecase.scheduledExecutionRepository.ListScheduledExecutionsOfOrganization(tx, organizationId)
			if err != nil {
				return []models.ScheduledExecution{}, err
			}
		} else {
			var err error
			executions, err = usecase.scheduledExecutionRepository.ListScheduledExecutionsOfScenario(tx, scenarioId)
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
	id := utils.NewPrimaryKey(input.OrganizationId)
	return usecase.transactionFactory.Transaction(models.DATABASE_MARBLE_SCHEMA, func(tx repositories.Transaction) error {
		return usecase.scheduledExecutionRepository.CreateScheduledExecution(tx, input, id)
	})
}

func (usecase *ScheduledExecutionUsecase) UpdateScheduledExecution(ctx context.Context, input models.UpdateScheduledExecutionInput) error {

	return usecase.transactionFactory.Transaction(models.DATABASE_MARBLE_SCHEMA, func(tx repositories.Transaction) error {
		execution, err := usecase.scheduledExecutionRepository.GetScheduledExecution(tx, input.Id)
		if err != nil {
			return err
		}
		if err := usecase.enforceSecurity.CreateScheduledExecution(execution.OrganizationId); err != nil {
			return err
		}
		return usecase.scheduledExecutionRepository.UpdateScheduledExecution(tx, input)
	})
}
