package repositories

import (
	"context"

	"github.com/Masterminds/squirrel"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories/dbmodels"
	"github.com/google/uuid"
)

func (repo *MarbleDbRepository) GetScoreHistory(
	ctx context.Context,
	exec Executor,
	record models.ScoringRecordRef,
) ([]models.ScoringScore, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, err
	}

	query := NewQueryBuilder().
		Select(dbmodels.SelectScoringScoresColumns...).
		From(dbmodels.TABLE_SCORING_SCORES).
		Where(squirrel.Eq{
			"org_id":      record.OrgId,
			"record_type": record.RecordType,
			"record_id":   record.RecordId,
		}).
		OrderBy("created_at")

	return SqlToListOfModels(ctx, exec, query, dbmodels.AdaptScoringScore)
}

func (repo *MarbleDbRepository) GetActiveScore(
	ctx context.Context,
	exec Executor,
	record models.ScoringRecordRef,
) (*models.ScoringScore, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, err
	}

	query := NewQueryBuilder().
		Select(dbmodels.SelectScoringScoresColumns...).
		From(dbmodels.TABLE_SCORING_SCORES).
		Where(squirrel.Eq{
			"org_id":      record.OrgId,
			"record_type": record.RecordType,
			"record_id":   record.RecordId,
		}).
		Where("deleted_at is null")

	return SqlToOptionalModel(ctx, exec, query, dbmodels.AdaptScoringScore)
}

func (repo *MarbleDbRepository) InsertScore(
	ctx context.Context,
	tx Transaction,
	req models.InsertScoreRequest,
) (models.ScoringScore, error) {
	if err := validateMarbleDbExecutor(tx); err != nil {
		return models.ScoringScore{}, err
	}

	if !req.IgnoredByCooldown {
		update := NewQueryBuilder().
			Update(dbmodels.TABLE_SCORING_SCORES).
			Set("deleted_at", squirrel.Expr("now()")).
			Where(squirrel.Eq{
				"org_id":      req.OrgId,
				"record_type": req.RecordType,
				"record_id":   req.RecordId,
			}).
			Where("deleted_at is null")

		if err := ExecBuilder(ctx, tx, update); err != nil {
			return models.ScoringScore{}, err
		}
	}

	deletedAt := squirrel.Expr("null")
	if req.IgnoredByCooldown {
		deletedAt = squirrel.Expr("now()")
	}

	insert := NewQueryBuilder().
		Insert(dbmodels.TABLE_SCORING_SCORES).
		Columns(
			"id",
			"org_id",
			"record_type",
			"record_id",
			"risk_level",
			"source",
			"ruleset_id",
			"overridden_by",
			"stale_at",
			"deleted_at",
		).
		Values(
			uuid.Must(uuid.NewV7()),
			req.OrgId,
			req.RecordType,
			req.RecordId,
			req.RiskLevel,
			req.Source,
			req.RulesetId,
			req.OverriddenBy,
			req.StaleAt,
			deletedAt,
		).
		Suffix("returning *")

	return SqlToModel(ctx, tx, insert, dbmodels.AdaptScoringScore)
}

func (repo *MarbleDbRepository) GetScoreDistribution(
	ctx context.Context,
	exec Executor,
	orgId uuid.UUID,
	recordType string,
) ([]models.ScoreDistribution, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, err
	}

	query := NewQueryBuilder().
		Select("risk_level", "count(*) as n").
		From(dbmodels.TABLE_SCORING_SCORES).
		Where(squirrel.Eq{
			"org_id":      orgId,
			"record_type": recordType,
			"source":      "ruleset",
			"deleted_at":  nil,
		}).
		GroupBy("risk_level")

	return SqlToListOfModels(ctx, exec, query, dbmodels.AdaptScoringScoreDistribution)
}
