package decision_workflows

import (
	"context"
	"fmt"
	"slices"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/pkg/errors"
)

type caseEditor interface {
	CreateCase(
		ctx context.Context,
		tx repositories.Executor,
		userId string,
		createCaseAttributes models.CreateCaseAttributes,
		fromEndUser bool,
	) (models.Case, error)
	UpdateDecisionsWithEvents(
		ctx context.Context,
		exec repositories.Executor,
		caseId, userId string,
		decisionIds []string,
	) error
}

type caseAndDecisionRepository interface {
	SelectCasesWithPivot(
		ctx context.Context,
		tx repositories.Executor,
		filters models.DecisionWorkflowFilters,
	) ([]models.CaseMetadata, error)
	CountDecisionsByCaseIds(ctx context.Context, tx repositories.Executor, caseIds []string) (map[string]int, error)
}

type DecisionsWorkflows struct {
	caseEditor caseEditor
	repository caseAndDecisionRepository
}

func NewDecisionWorkflows(
	caseEditor caseEditor,
	repository caseAndDecisionRepository,
) DecisionsWorkflows {
	return DecisionsWorkflows{
		caseEditor: caseEditor,
		repository: repository,
	}
}

func (d DecisionsWorkflows) AutomaticDecisionToCase(
	ctx context.Context,
	tx repositories.Executor,
	scenario models.Scenario,
	decision models.DecisionWithRuleExecutions,
) error {
	if scenario.DecisionToCaseWorkflowType == models.WorkflowDisabled ||
		scenario.DecisionToCaseOutcomes == nil ||
		!slices.Contains(scenario.DecisionToCaseOutcomes, decision.Outcome) ||
		scenario.DecisionToCaseInboxId == nil {
		return nil
	}

	if scenario.DecisionToCaseWorkflowType == models.WorkflowCreateCase {
		input := automaticCreateCaseAttributes(scenario, decision)
		_, err := d.caseEditor.CreateCase(ctx, tx, "", input, false)
		if err != nil {
			return errors.Wrap(err, "error creating case for decision")
		}
		return nil
	}

	if scenario.DecisionToCaseWorkflowType == models.WorkflowAddToCaseIfPossible {
		added, err := d.addToOpenCase(ctx, tx, scenario, decision)
		if err != nil {
			return errors.Wrap(err, "error adding decision to open case")
		}

		if !added {
			input := automaticCreateCaseAttributes(scenario, decision)
			_, err := d.caseEditor.CreateCase(ctx, tx, "", input, false)
			if err != nil {
				return errors.Wrap(err, "error creating case for decision")
			}
		}
		return nil
	}

	return errors.New(fmt.Sprintf("unknown workflow type: %s", scenario.DecisionToCaseWorkflowType))
}

func automaticCreateCaseAttributes(
	scenario models.Scenario,
	decision models.DecisionWithRuleExecutions,
) models.CreateCaseAttributes {
	return models.CreateCaseAttributes{
		DecisionIds: []string{decision.DecisionId},
		InboxId:     *scenario.DecisionToCaseInboxId,
		Name: fmt.Sprintf(
			"Case for %s: %s",
			scenario.TriggerObjectType,
			decision.ClientObject.Data["object_id"],
		),
		OrganizationId: scenario.OrganizationId,
	}
}

func (d DecisionsWorkflows) addToOpenCase(
	ctx context.Context,
	tx repositories.Executor,
	scenario models.Scenario,
	decision models.DecisionWithRuleExecutions,
) (bool, error) {
	eligibleCases, err := d.repository.SelectCasesWithPivot(ctx, tx, models.DecisionWorkflowFilters{
		InboxId:        *scenario.DecisionToCaseInboxId,
		OrganizationId: scenario.OrganizationId,
		PivotValue:     *decision.PivotValue,
		Status:         []models.CaseStatus{models.CaseOpen, models.CaseInvestigating},
	})
	if err != nil {
		return false, errors.Wrap(err, "error selecting cases with pivot")
	}

	if len(eligibleCases) == 0 {
		return false, nil
	}

	caseIds := make([]string, 0, len(eligibleCases))
	for _, c := range eligibleCases {
		caseIds = append(caseIds, c.Id)
	}

	decisionCounts, err := d.repository.CountDecisionsByCaseIds(ctx, tx, caseIds)
	if err != nil {
		return false, errors.Wrap(err, "error counting decisions by case ids")
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
		return false, errors.Wrap(err, "error updating case")
	}

	return true, nil
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
	if a.DecisionCount < b.DecisionCount {
		// b contains more decisions with pivot values than a (in particular, more than one)
		return true
	}

	if a.Status == models.CaseOpen && b.Status == models.CaseInvestigating {
		// a is open, b is investigating
		return true
	}

	return a.CreatedAt.After(b.CreatedAt)
}
