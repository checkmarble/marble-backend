package decision_workflows

import (
	"context"
	"fmt"
	"slices"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/pkg/errors"
)

type caseCreator interface {
	CreateCase(
		ctx context.Context,
		tx repositories.Executor,
		userId string,
		createCaseAttributes models.CreateCaseAttributes,
		fromEndUser bool,
	) (models.Case, error)
}

type caseAndDecisionRepository interface{}

type DecisionsWorkflows struct {
	caseCreator caseCreator
	repository  caseAndDecisionRepository
}

func NewDecisionWorkflows(
	caseCreator caseCreator,
	repository caseAndDecisionRepository,
) DecisionsWorkflows {
	return DecisionsWorkflows{
		caseCreator: caseCreator,
		repository:  repository,
	}
}

func (d DecisionsWorkflows) CreateCaseIfApplicable(
	ctx context.Context,
	tx repositories.Executor,
	scenario models.Scenario,
	decision models.DecisionWithRuleExecutions,
) error {
	if scenario.DecisionToCaseOutcomes != nil &&
		slices.Contains(scenario.DecisionToCaseOutcomes, decision.Outcome) &&
		scenario.DecisionToCaseInboxId != nil {
		input := models.CreateCaseAttributes{
			DecisionIds: []string{decision.DecisionId},
			InboxId:     *scenario.DecisionToCaseInboxId,
			Name: fmt.Sprintf(
				"Case for %s: %s",
				scenario.TriggerObjectType,
				decision.ClientObject.Data["object_id"],
			),
			OrganizationId: scenario.OrganizationId,
		}
		_, err := d.caseCreator.CreateCase(ctx, tx, "", input, false)
		if err != nil {
			return errors.Wrap(err, "error creating case for decision")
		}
	}
	return nil
}
