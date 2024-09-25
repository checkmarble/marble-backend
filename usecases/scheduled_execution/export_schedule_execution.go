package scheduled_execution

import (
	"context"
	"encoding/json"
	"io"

	"github.com/cockroachdb/errors"

	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
)

type ExportScheduleExecution struct {
	DecisionRepository     repositories.DecisionRepository
	OrganizationRepository repositories.OrganizationRepository
	ExecutorFactory        executor_factory.ExecutorFactory
}

func (exporter *ExportScheduleExecution) ExportDecisions(
	ctx context.Context,
	organizationId string,
	scheduledExecutionId string,
	dest io.Writer,
) (int, error) {
	decisionChan, errorChan := exporter.DecisionRepository.DecisionsOfScheduledExecution(
		ctx,
		exporter.ExecutorFactory.NewExecutor(),
		organizationId,
		scheduledExecutionId,
	)

	encoder := json.NewEncoder(dest)

	var allErrors []error

	var number_of_exported_decisions int

	for decision := range decisionChan {
		if decision.OrganizationId != organizationId {
			allErrors = append(
				allErrors,
				errors.Wrap(models.ForbiddenError, "decision does not belong to the organization"),
			)
			return number_of_exported_decisions, errors.Join(allErrors...)
		}
		err := encoder.Encode(dto.NewDecisionWithRuleDto(decision, "", false))
		if err != nil {
			allErrors = append(allErrors, err)
		} else {
			number_of_exported_decisions += 1
		}
	}

	// wait for DecisionsOfScheduledExecution to finish
	err := <-errorChan

	return number_of_exported_decisions, errors.Join(append(allErrors, err)...)
}
