package decision_workflows

import (
	"context"
	"fmt"

	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/models/ast"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/evaluate_scenario"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/google/uuid"
	"github.com/pkg/errors"
)

func (d DecisionsWorkflows) AutomaticDecisionToCase(
	ctx context.Context,
	tx repositories.Transaction,
	scenario models.Scenario,
	decision models.DecisionWithRuleExecutions,
	evalParams evaluate_scenario.ScenarioEvaluationParameters,
	action models.WorkflowActionSpec[dto.WorkflowActionCaseParams],
) (models.WorkflowExecution, error) {
	logger := utils.LoggerFromContext(ctx)
	webhookEventId := uuid.NewString()
	orgId := evalParams.Scenario.OrganizationId

	createNewCaseForDecision := func(ctx context.Context) (models.WorkflowExecution, error) {
		var titleTemplateAst *ast.Node

		if action.Params.TitleTemplate != nil {
			astNode, err := dto.AdaptASTNode(*action.Params.TitleTemplate)
			if err != nil {
				return models.WorkflowExecution{}, err
			}

			titleTemplateAst = &astNode
		}

		caseName, err := d.caseNameEvaluator.EvalCaseName(ctx, evalParams, scenario, titleTemplateAst)
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
			OrganizationId: orgId,
			EventContent:   models.NewWebhookEventCaseCreatedWorkflow(newCase.GetMetadata()),
		})
		if err != nil {
			return models.WorkflowExecution{}, errors.Wrap(err, "error creating webhook event")
		}

		caseId, err := uuid.Parse(newCase.Id)
		if err != nil {
			return models.WorkflowExecution{}, errors.Wrap(err, "error parsing case id")
		}

		hasAiCaseReviewEnabled, err := d.aiAgentUsecase.HasAiCaseReviewEnabled(ctx, newCase.OrganizationId)
		if err != nil {
			return models.WorkflowExecution{}, errors.Wrap(err,
				"error checking if AI case review is enabled")
		}
		if hasAiCaseReviewEnabled {
			inbox, err := d.repository.GetInboxById(ctx, tx, newCase.InboxId)
			if err != nil {
				return models.WorkflowExecution{}, errors.Wrap(err, "error getting inbox")
			}
			if inbox.CaseReviewOnCaseCreated {
				caseReviewId := uuid.Must(uuid.NewV7())
				err = d.caseReviewTaskEnqueuer.EnqueueCaseReviewTask(ctx, tx, orgId, caseId, caseReviewId)
				if err != nil {
					return models.WorkflowExecution{}, errors.Wrap(err, "error enqueuing case review task")
				}
			}
		}

		return models.WorkflowExecution{
			AddedToCase: true,
			WebhookIds:  []string{webhookEventId},
		}, nil
	}

	switch action.Action {
	case models.WorkflowCreateCase:
		return createNewCaseForDecision(ctx)
	case models.WorkflowAddToCaseIfPossible:
		// Get an advisory lock on pivot value to prevent race conditions when multiple decisions
		// for the same entity (user, account, etc.) arrive simultaneously. This ensures that
		// AddToCaseIfPossible operations are serialized and only one case gets created per pivot value.
		// If pivot value is nil, no lock is needed since we'll always create a new case.
		if decision.PivotValue != nil {
			logger.DebugContext(
				ctx,
				"getting advisory lock on pivot",
				"pivot_value", *decision.PivotValue,
				"inbox_id", action.Params.InboxId,
			)
			err := repositories.GetAdvisoryLockTx(ctx, tx, lockKey(decision, action))
			if err != nil {
				return models.WorkflowExecution{}, errors.Wrap(err,
					"error getting advisory lock on pivot value")
			}
			logger.DebugContext(
				ctx,
				"acquired advisory lock on pivot",
				"pivot_value", *decision.PivotValue,
				"inbox_id", action.Params.InboxId,
			)
		}

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
		return models.WorkflowExecution{}, errors.New(
			fmt.Sprintf("unknown workflow type: %s", action.Action))
	}
}

func automaticCreateCaseAttributes(
	scenario models.Scenario,
	decision models.DecisionWithRuleExecutions,
	action models.WorkflowActionSpec[dto.WorkflowActionCaseParams],
	name string,
) models.CreateCaseAttributes {
	return models.CreateCaseAttributes{
		DecisionIds:    []string{decision.DecisionId.String()},
		InboxId:        action.Params.InboxId,
		Name:           name,
		OrganizationId: scenario.OrganizationId,
	}
}

func (d DecisionsWorkflows) addToOpenCase(
	ctx context.Context,
	tx repositories.Transaction,
	scenario models.Scenario,
	decision models.DecisionWithRuleExecutions,
	action models.WorkflowActionSpec[dto.WorkflowActionCaseParams],
) (models.CaseMetadata, bool, error) {
	if decision.PivotValue == nil {
		return models.CaseMetadata{}, false, nil
	}

	filters := models.DecisionWorkflowFilters{
		OrganizationId: scenario.OrganizationId,
		PivotValue:     *decision.PivotValue,
	}

	if !action.Params.AnyInbox {
		filters.InboxId = &action.Params.InboxId
	}

	eligibleCases, err := d.repository.SelectCasesWithPivot(ctx, tx, filters)
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

// Build a key for advisory lock on pivot value, pivot ID and inbox ID
func lockKey(decision models.DecisionWithRuleExecutions, action models.WorkflowActionSpec[dto.WorkflowActionCaseParams]) string {
	pivotValue := ""
	if decision.PivotValue != nil {
		pivotValue = *decision.PivotValue
	}
	pivotId := uuid.UUID{}
	if decision.PivotId != nil {
		pivotId = *decision.PivotId
	}
	inboxId := uuid.UUID{}
	if !action.Params.AnyInbox {
		inboxId = action.Params.InboxId
	}
	return fmt.Sprintf("%s-%s-%s", pivotValue, pivotId.String(), inboxId.String())
}
