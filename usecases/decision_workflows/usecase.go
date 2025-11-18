package decision_workflows

import (
	"context"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/models/ast"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/ast_eval"
	"github.com/checkmarble/marble-backend/usecases/evaluate_scenario"
	"github.com/google/uuid"
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
		orgId uuid.UUID,
		caseId, userId string,
		decisionIdsToAdd []string,
	) error
}

type decisionWorkflowsRepository interface {
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
	GetInboxById(ctx context.Context, exec repositories.Executor, inboxId uuid.UUID) (models.Inbox, error)
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

type caseReviewTaskEnqueuer interface {
	EnqueueCaseReviewTask(
		ctx context.Context,
		tx repositories.Transaction,
		organizationId string,
		caseId uuid.UUID,
		aiCaseReviewId uuid.UUID,
	) error
}

type aiAgentUsecase interface {
	HasAiCaseReviewEnabled(ctx context.Context, orgId string) (bool, error)
}

type DecisionsWorkflows struct {
	repository             decisionWorkflowsRepository
	caseEditor             caseEditor
	caseNameEvaluator      CaseNameEvaluator
	webhookEventCreator    webhookEventCreator
	astEvaluator           ast_eval.EvaluateAstExpression
	caseReviewTaskEnqueuer caseReviewTaskEnqueuer
	caseManagerBucketUrl   string
	aiAgentUsecase         aiAgentUsecase
}

func NewDecisionWorkflows(
	caseEditor caseEditor,
	repository decisionWorkflowsRepository,
	webhookEventCreator webhookEventCreator,
	caseNameEvaluator CaseNameEvaluator,
	astEvaluator ast_eval.EvaluateAstExpression,
	caseReviewTaskEnqueuer caseReviewTaskEnqueuer,
	caseManagerBucketUrl string,
	aiAgentUsecase aiAgentUsecase,
) DecisionsWorkflows {
	return DecisionsWorkflows{
		caseEditor:             caseEditor,
		repository:             repository,
		webhookEventCreator:    webhookEventCreator,
		caseNameEvaluator:      caseNameEvaluator,
		astEvaluator:           astEvaluator,
		caseReviewTaskEnqueuer: caseReviewTaskEnqueuer,
		caseManagerBucketUrl:   caseManagerBucketUrl,
		aiAgentUsecase:         aiAgentUsecase,
	}
}
