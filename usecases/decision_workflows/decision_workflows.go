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
	SelectCasesWithPivot(ctx context.Context, tx repositories.Executor, pivotValue string) ([]models.CaseMetadata, error)
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
		_, err := d.caseEditor.CreateCase(ctx, tx, "", input, false)
		if err != nil {
			return errors.Wrap(err, "error creating case for decision")
		}
	}
	return nil
}

func (d DecisionsWorkflows) AddToCaseIfAnyOpen(
	ctx context.Context,
	tx repositories.Executor,
	scenario models.Scenario,
	decision models.DecisionWithRuleExecutions,
) error {
	if decision.PivotValue == nil {
		return nil
	}

	eligibleCases, err := d.repository.SelectCasesWithPivot(ctx, tx, *decision.PivotValue)
	if err != nil {
		return errors.Wrap(err, "error selecting cases with pivot")
	}

	if len(eligibleCases) == 0 {
		return nil
	}

	caseIds := make([]string, 0, len(eligibleCases))
	for _, c := range eligibleCases {
		caseIds = append(caseIds, c.Id)
	}

	decisionCounts, err := d.repository.CountDecisionsByCaseIds(ctx, tx, caseIds)
	if err != nil {
		return errors.Wrap(err, "error counting decisions by case ids")
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
		return errors.Wrap(err, "error updating case")
	}

	return nil
}

type caseMetadataWithDecisionCount struct {
	models.CaseMetadata
	DecisionCount int
}

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
	// TODO implement the real logic
	return a.DecisionCount < b.DecisionCount
}
