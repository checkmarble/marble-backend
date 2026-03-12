package scoring

import (
	"context"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/google/uuid"
)

type ScoringRepository interface {
	GetScoringSettings(ctx context.Context, exec repositories.Executor, orgId uuid.UUID) (*models.ScoringSettings, error)
	UpdateScoringSettings(ctx context.Context, exec repositories.Executor, settings models.ScoringSettings) (models.ScoringSettings, error)

	ListScoringRulesets(ctx context.Context, exec repositories.Executor, orgId uuid.UUID) ([]models.ScoringRuleset, error)
	GetScoringRuleset(ctx context.Context, exec repositories.Executor, orgId uuid.UUID, recordType string, status models.ScoreRulesetStatus, version int) (models.ScoringRuleset, error)
	ListScoringRulesetVersions(
		ctx context.Context,
		exec repositories.Executor,
		orgId uuid.UUID,
		recordType string,
	) ([]models.ScoringRuleset, error)
	GetScoringRulesetById(ctx context.Context, exec repositories.Executor, orgId, id uuid.UUID) (models.ScoringRuleset, error)
	InsertScoringRulesetVersion(ctx context.Context, exec repositories.Transaction,
		orgId uuid.UUID,
		ruleset models.CreateScoringRulesetRequest,
	) (models.ScoringRuleset, error)
	InsertScoringRulesetVersionRule(ctx context.Context, tx repositories.Transaction,
		ruleset models.ScoringRuleset,
		rules []models.CreateScoringRuleRequest,
	) ([]models.ScoringRule, error)
	CommitRuleset(ctx context.Context, exec repositories.Executor, ruleset models.ScoringRuleset) (models.ScoringRuleset, error)

	GetScoreHistory(ctx context.Context, exec repositories.Executor, record models.ScoringRecordRef) ([]models.ScoringScore, error)
	GetActiveScore(ctx context.Context, exec repositories.Executor, record models.ScoringRecordRef) (*models.ScoringScore, error)
	InsertScore(ctx context.Context, tx repositories.Transaction, req models.InsertScoreRequest) (models.ScoringScore, error)

	GetScoreDistribution(ctx context.Context, exec repositories.Executor, orgId uuid.UUID, entityType string) ([]models.ScoreDistribution, error)

	GetScoringLatestDryRun(
		ctx context.Context,
		exec repositories.Executor,
		rulesetId uuid.UUID,
	) (models.ScoringDryRun, error)
	GetScoringDryRunById(
		ctx context.Context,
		exec repositories.Executor,
		id uuid.UUID,
	) (models.ScoringDryRun, error)
	InsertRulesetDryRun(
		ctx context.Context,
		exec repositories.Transaction,
		ruleset models.ScoringRuleset,
		objectCount int,
	) (models.ScoringDryRun, error)
	CancelRulesetDryRun(
		ctx context.Context,
		exec repositories.Executor,
		ruleset models.ScoringRuleset,
	) error
	SetRulesetDryRunStatus(
		ctx context.Context,
		exec repositories.Executor,
		dryRun models.ScoringDryRun,
		status models.DryRunStatus,
		results map[int]int,
	) (*models.ScoringDryRun, error)

	GetStaleScoreBatch(
		ctx context.Context,
		exec repositories.Executor,
		orgId uuid.UUID,
		recordType string,
		before time.Time,
		limit int,
	) ([]string, error)
}

type scoringIngestedDataReader interface {
	QueryIngestedObject(ctx context.Context, exec repositories.Executor,
		table models.Table, objectId string, metadataFields ...string) ([]models.DataModelObject, error)
}

type scoringIndexEditor interface {
	GetIndexesToCreateForScoringRuleset(ctx context.Context, organizationId uuid.UUID, ruleset models.ScoringRuleset) (toCreate []models.ConcreteIndex, numPending int, err error)
	CreateIndexesAsync(ctx context.Context, organizationId uuid.UUID, indexes []models.ConcreteIndex) error
}
