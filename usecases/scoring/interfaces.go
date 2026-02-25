package scoring

import (
	"context"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/google/uuid"
)

type scoringRepository interface {
	ListScoringRulesets(ctx context.Context, exec repositories.Executor, orgId uuid.UUID) ([]models.ScoringRuleset, error)
	GetScoringRuleset(ctx context.Context, exec repositories.Executor, orgId uuid.UUID, entityType string) (models.ScoringRuleset, error)
	InsertScoringRulesetVersion(ctx context.Context, exec repositories.Transaction,
		orgId uuid.UUID,
		ruleset models.CreateScoringRulesetRequest,
	) (models.ScoringRuleset, error)
	InsertScoringRulesetVersionRule(ctx context.Context, tx repositories.Transaction,
		ruleset models.ScoringRuleset,
		rule models.CreateScoringRuleRequest,
	) (models.ScoringRule, error)

	GetScoreHistory(ctx context.Context, exec repositories.Executor, entityRef models.ScoringEntityRef) ([]models.ScoringScore, error)
	GetActiveScore(ctx context.Context, exec repositories.Executor, entityRef models.ScoringEntityRef) (*models.ScoringScore, error)
	InsertScore(ctx context.Context, tx repositories.Transaction, req models.InsertScoreRequest) (models.ScoringScore, error)
}

type scoringIngestedDataReader interface {
	QueryIngestedObject(ctx context.Context, exec repositories.Executor,
		table models.Table, objectId string, metadataFields ...string) ([]models.DataModelObject, error)
}
