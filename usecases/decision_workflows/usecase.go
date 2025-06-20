package decision_workflows

import (
	"context"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/models/ast"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/evaluate_scenario"
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
		scenario models.Scenario, titleTemplate *ast.Node) (string, error)
}

type DecisionsWorkflows struct {
	repository          caseAndDecisionRepository
	caseEditor          caseEditor
	caseNameEvaluator   CaseNameEvaluator
	webhookEventCreator webhookEventCreator
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
