package repositories

import (
	"context"
	"time"

	"github.com/Masterminds/squirrel"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
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
			pure_utils.NewId(),
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

func (repo *MarbleDbRepository) InsertEmptyScore(
	ctx context.Context,
	exec Executor,
	req models.InsertScoreRequest,
) error {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return err
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
			pure_utils.NewId(),
			req.OrgId,
			req.RecordType,
			req.RecordId,
			0,
			models.ScoreSourceInitial,
			req.RulesetId,
			nil,
			nil,
			nil,
		).
		Suffix("on conflict do nothing")

	return ExecBuilder(ctx, exec, insert)
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

func (repo *MarbleDbRepository) GetStaleScoreBatch(
	ctx context.Context,
	exec Executor,
	orgId uuid.UUID,
	recordType string,
	before time.Time,
	limit int,
) ([]string, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, err
	}

	query := NewQueryBuilder().
		Select("record_id").
		From(dbmodels.TABLE_SCORING_SCORES).
		Where(squirrel.Eq{
			"org_id":      orgId,
			"record_type": recordType,
			"deleted_at":  nil,
		}).
		Where(squirrel.Or{
			squirrel.And{
				squirrel.Eq{"source": "ruleset"},
				squirrel.Expr("created_at < ?", before),
			},
			squirrel.And{
				squirrel.Eq{"source": "override"},
				squirrel.Expr("stale_at < now()"),
			},
		}).
		OrderBy("created_at asc").
		Limit(uint64(limit))

	sql, args, err := query.ToSql()
	if err != nil {
		return nil, err
	}

	rows, err := exec.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	recordIds := make([]string, 0, limit)

	var tmp string

	for rows.Next() {
		if err := rows.Scan(&tmp); err != nil {
			return nil, err
		}

		recordIds = append(recordIds, tmp)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return recordIds, nil
}
