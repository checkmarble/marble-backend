package decision_workflows

import (
	"context"
	"fmt"
	"slices"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/evaluate_scenario"
	"github.com/pkg/errors"
)

type caseEditor interface {
	CreateCase(
		ctx context.Context,
		tx repositories.Transaction,
		userId string,
		createCaseAttributes models.CreateCaseAttributes,
		fromEndUser bool,
	) (models.Case, error)
	UpdateDecisionsWithEvents(
		ctx context.Context,
		exec repositories.Executor,
		caseId, userId string,
		decisionIdsToAdd []string,
	) error
}

type caseAndDecisionRepository interface {
	SelectCasesWithPivot(
		ctx context.Context,
		exec repositories.Executor,
		filters models.DecisionWorkflowFilters,
	) ([]models.CaseMetadata, error)
	CountDecisionsByCaseIds(
		ctx context.Context,
		exec repositories.Executor,
		organizationId string,
		caseIds []string,
	) (map[string]int, error)
}

type webhookEventCreator interface {
	CreateWebhookEvent(
		ctx context.Context,
		tx repositories.Transaction,
		create models.WebhookEventCreate,
	) error
}

type CaseNameEvaluator interface {
	EvalCaseName(ctx context.Context, params evaluate_scenario.ScenarioEvaluationParameters,
		scenario models.Scenario) (string, error)
}

type DecisionsWorkflows struct {
	caseEditor          caseEditor
	repository          caseAndDecisionRepository
	webhookEventCreator webhookEventCreator
	caseNameEvaluator   CaseNameEvaluator
}

func NewDecisionWorkflows(
	caseEditor caseEditor,
	repository caseAndDecisionRepository,
	webhookEventCreator webhookEventCreator,
	caseNameEvaluator CaseNameEvaluator,
) DecisionsWorkflows {
	return DecisionsWorkflows{
		caseEditor:          caseEditor,
		repository:          repository,
		webhookEventCreator: webhookEventCreator,
		caseNameEvaluator:   caseNameEvaluator,
	}
}

func (d DecisionsWorkflows) AutomaticDecisionToCase(
	ctx context.Context,
	tx repositories.Transaction,
	scenario models.Scenario,
	decision models.DecisionWithRuleExecutions,
	params evaluate_scenario.ScenarioEvaluationParameters,
	webhookEventId string,
) (addedToCase bool, err error) {
	if scenario.DecisionToCaseWorkflowType == models.WorkflowDisabled ||
		scenario.DecisionToCaseOutcomes == nil ||
		!slices.Contains(scenario.DecisionToCaseOutcomes, decision.Outcome) ||
		scenario.DecisionToCaseInboxId == nil {
		return false, nil
	}

	if scenario.DecisionToCaseWorkflowType == models.WorkflowCreateCase {
		caseName, err := d.caseNameEvaluator.EvalCaseName(ctx, params, scenario)
		if err != nil {
			return false, errors.Wrap(err, "error creating case for decision")
		}

		input := automaticCreateCaseAttributes(scenario, decision, caseName)
		newCase, err := d.caseEditor.CreateCase(ctx, tx, "", input, false)
		if err != nil {
			return false, errors.Wrap(err, "error creating case for decision")
		}
		err = d.webhookEventCreator.CreateWebhookEvent(ctx, tx, models.WebhookEventCreate{
			Id:             webhookEventId,
			OrganizationId: newCase.OrganizationId,
			EventContent:   models.NewWebhookEventCaseCreatedWorkflow(newCase.GetMetadata()),
		})
		if err != nil {
			return false, err
		}

		return true, nil
	}

	if scenario.DecisionToCaseWorkflowType == models.WorkflowAddToCaseIfPossible {
		matchedCase, added, err := d.addToOpenCase(ctx, tx, scenario, decision)
		if err != nil {
			return false, errors.Wrap(err, "error adding decision to open case")
		}

		if !added {
			caseName, err := d.caseNameEvaluator.EvalCaseName(ctx, params, scenario)
			if err != nil {
				return false, errors.Wrap(err, "error creating case for decision")
			}

			input := automaticCreateCaseAttributes(scenario, decision, caseName)
			newCase, err := d.caseEditor.CreateCase(ctx, tx, "", input, false)
			if err != nil {
				return false, errors.Wrap(err, "error creating case for decision")
			}

			err = d.webhookEventCreator.CreateWebhookEvent(ctx, tx, models.WebhookEventCreate{
				Id:             webhookEventId,
				OrganizationId: newCase.OrganizationId,
				EventContent:   models.NewWebhookEventCaseCreatedWorkflow(newCase.GetMetadata()),
			})
			if err != nil {
				return false, err
			}

			return true, nil
		}

		err = d.webhookEventCreator.CreateWebhookEvent(ctx, tx, models.WebhookEventCreate{
			Id:             webhookEventId,
			OrganizationId: matchedCase.OrganizationId,
			EventContent:   models.NewWebhookEventCaseDecisionsUpdated(matchedCase),
		})
		if err != nil {
			return false, err
		}

		return true, nil
	}

	return false, errors.New(fmt.Sprintf("unknown workflow type: %s", scenario.DecisionToCaseWorkflowType))
}

func automaticCreateCaseAttributes(
	scenario models.Scenario,
	decision models.DecisionWithRuleExecutions,
	name string,
) models.CreateCaseAttributes {
	return models.CreateCaseAttributes{
		DecisionIds:    []string{decision.DecisionId},
		InboxId:        *scenario.DecisionToCaseInboxId,
		Name:           name,
		OrganizationId: scenario.OrganizationId,
	}
}

func (d DecisionsWorkflows) addToOpenCase(
	ctx context.Context,
	tx repositories.Transaction,
	scenario models.Scenario,
	decision models.DecisionWithRuleExecutions,
) (models.CaseMetadata, bool, error) {
	if decision.PivotValue == nil {
		return models.CaseMetadata{}, false, nil
	}

	eligibleCases, err := d.repository.SelectCasesWithPivot(ctx, tx, models.DecisionWorkflowFilters{
		InboxId:        *scenario.DecisionToCaseInboxId,
		OrganizationId: scenario.OrganizationId,
		PivotValue:     *decision.PivotValue,
	})
	if err != nil {
		return models.CaseMetadata{}, false, errors.Wrap(err, "error selecting cases with pivot")
	}

	if len(eligibleCases) == 0 {
		return models.CaseMetadata{}, false, nil
	}

	caseIds := make([]string, 0, len(eligibleCases))
	for _, c := range eligibleCases {
		caseIds = append(caseIds, c.Id)
	}

	decisionCounts, err := d.repository.CountDecisionsByCaseIds(ctx, tx, scenario.OrganizationId, caseIds)
	if err != nil {
		return models.CaseMetadata{}, false, errors.Wrap(err, "error counting decisions by case ids")
	}

	cases := make([]caseMetadataWithDecisionCount, 0, len(eligibleCases))
	for _, c := range eligibleCases {
		cases = append(cases, caseMetadataWithDecisionCount{
			CaseMetadata:  c,
			DecisionCount: decisionCounts[c.Id],
		})
	}

	bestMatchCase := findBestMatchCase(cases)
	err = d.caseEditor.UpdateDecisionsWithEvents(ctx, tx, bestMatchCase.Id, "", []string{decision.DecisionId})
	if err != nil {
		return models.CaseMetadata{}, false, errors.Wrap(err, "error updating case")
	}

	return bestMatchCase, true, nil
}

type caseMetadataWithDecisionCount struct {
	models.CaseMetadata
	DecisionCount int
}

// The best match:
// - has only this the pivot value common to all the cases, if possible (at least, has the least possible number of distinct pivot values)
// - is open rather than investigating, if possible
// - if everyting else is equal, is the most recent case
// (we know implicitly that all the cases share at least one common pivot value)
func findBestMatchCase(cases []caseMetadataWithDecisionCount) models.CaseMetadata {
	bestMatch := cases[0]
	for _, c := range cases {
		if caseIsBetterMatch(c, bestMatch) {
			bestMatch = c
		}
	}
	return bestMatch.CaseMetadata
}

func caseIsBetterMatch(a, b caseMetadataWithDecisionCount) bool {
	if a.DecisionCount != b.DecisionCount {
		// a has fewer distinct pivot values than b - particular case if a has only one distinct pivot value (all have at least one)
		return a.DecisionCount < b.DecisionCount
	}

	if a.Status != b.Status {
		return a.Status == models.CasePending
	}

	return a.CreatedAt.After(b.CreatedAt)
}
