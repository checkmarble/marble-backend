package decision_workflows

import (
	"context"
	"fmt"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/evaluate_scenario"
	"github.com/google/uuid"
	"github.com/pkg/errors"
)

func (d DecisionsWorkflows) AutomaticDecisionToCase(
	ctx context.Context,
	tx repositories.Transaction,
	scenario models.Scenario,
	decision models.DecisionWithRuleExecutions,
	evalParams evaluate_scenario.ScenarioEvaluationParameters,
	action models.WorkflowActionSpec[models.WorkflowCaseParams],
) (models.WorkflowExecution, error) {
	if action.Params.InboxId == nil {
		return models.WorkflowExecution{}, nil
	}

	webhookEventId := uuid.NewString()

	createNewCaseForDecision := func(ctx context.Context) (models.WorkflowExecution, error) {
		caseName, err := d.caseNameEvaluator.EvalCaseName(ctx, evalParams, scenario, action.Params.TitleTemplate)
		if err != nil {
			return models.WorkflowExecution{}, errors.Wrap(err, "error creating case for decision")
		}

		input := automaticCreateCaseAttributes(scenario, decision, action, caseName)
		newCase, err := d.caseEditor.CreateCase(ctx, tx, "", input, false)
		if err != nil {
			return models.WorkflowExecution{}, errors.Wrap(err, "error creating case for decision")
		}

		err = d.webhookEventCreator.CreateWebhookEvent(ctx, tx, models.WebhookEventCreate{
			Id:             webhookEventId,
			OrganizationId: newCase.OrganizationId,
			EventContent:   models.NewWebhookEventCaseCreatedWorkflow(newCase.GetMetadata()),
		})
		return models.WorkflowExecution{
			WebhookIds:  []string{webhookEventId},
			AddedToCase: true,
		}, err
	}

	switch action.Action {
	case models.WorkflowCreateCase:
		return createNewCaseForDecision(ctx)
	case models.WorkflowAddToCaseIfPossible:
		matchedCase, added, err := d.addToOpenCase(ctx, tx, scenario, decision, action)
		if err != nil {
			return models.WorkflowExecution{}, errors.Wrap(err, "error adding decision to open case")
		}
		if !added {
			return createNewCaseForDecision(ctx)
		}

		err = d.webhookEventCreator.CreateWebhookEvent(ctx, tx, models.WebhookEventCreate{
			Id:             webhookEventId,
			OrganizationId: matchedCase.OrganizationId,
			EventContent:   models.NewWebhookEventCaseDecisionsUpdated(matchedCase),
		})
		if err != nil {
			return models.WorkflowExecution{}, err
		}

		return models.WorkflowExecution{
			AddedToCase: true,
			WebhookIds:  []string{webhookEventId},
		}, nil
	default:
		return models.WorkflowExecution{}, errors.New(fmt.Sprintf("unknown workflow type: %s", action.Action))
	}
}

func automaticCreateCaseAttributes(
	scenario models.Scenario,
	decision models.DecisionWithRuleExecutions,
	action models.WorkflowActionSpec[models.WorkflowCaseParams],
	name string,
) models.CreateCaseAttributes {
	return models.CreateCaseAttributes{
		DecisionIds:    []string{decision.DecisionId.String()},
		InboxId:        *action.Params.InboxId,
		Name:           name,
		OrganizationId: scenario.OrganizationId,
	}
}

func (d DecisionsWorkflows) addToOpenCase(
	ctx context.Context,
	tx repositories.Transaction,
	scenario models.Scenario,
	decision models.DecisionWithRuleExecutions,
	action models.WorkflowActionSpec[models.WorkflowCaseParams],
) (models.CaseMetadata, bool, error) {
	if decision.PivotValue == nil {
		return models.CaseMetadata{}, false, nil
	}

	eligibleCases, err := d.repository.SelectCasesWithPivot(ctx, tx, models.DecisionWorkflowFilters{
		InboxId:        *action.Params.InboxId,
		OrganizationId: scenario.OrganizationId,
		PivotValue:     *decision.PivotValue,
	})
	if err != nil {
		return models.CaseMetadata{}, false, errors.Wrap(err, "error selecting cases with pivot")
	}

	var bestMatchCase models.CaseMetadata
	switch len(eligibleCases) {
	case 0:
		return models.CaseMetadata{}, false, nil
	case 1:
		bestMatchCase = eligibleCases[0]
	default:
		caseIds := make([]string, len(eligibleCases))
		for i, c := range eligibleCases {
			caseIds[i] = c.Id
		}

		decisionCounts, err := d.repository.CountDecisionsByCaseIds(ctx, tx, scenario.OrganizationId, caseIds)
		if err != nil {
			return models.CaseMetadata{}, false, errors.Wrap(err, "error counting decisions by case ids")
		}

		cases := make([]caseMetadataWithDecisionCount, len(eligibleCases))
		for i, c := range eligibleCases {
			cases[i] = caseMetadataWithDecisionCount{
				CaseMetadata:  c,
				DecisionCount: decisionCounts[c.Id],
			}
		}
		bestMatchCase = findBestMatchCase(cases)
	}
	err = d.caseEditor.UpdateDecisionsWithEvents(ctx, tx, bestMatchCase.Id, "", []string{decision.DecisionId.String()})
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
