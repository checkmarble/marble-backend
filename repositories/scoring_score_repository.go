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
	entityRef models.ScoringEntityRef,
) ([]models.ScoringScore, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, err
	}

	query := NewQueryBuilder().
		Select(dbmodels.SelectScoringScoresColumns...).
		From(dbmodels.TABLE_SCORING_SCORES).
		Where(squirrel.Eq{
			"org_id":      entityRef.OrgId,
			"entity_type": entityRef.EntityType,
			"entity_id":   entityRef.EntityId,
		}).
		OrderBy("created_at")

	return SqlToListOfModels(ctx, exec, query, dbmodels.AdaptScoringScore)
}

func (repo *MarbleDbRepository) GetActiveScore(
	ctx context.Context,
	exec Executor,
	entityRef models.ScoringEntityRef,
) (*models.ScoringScore, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, err
	}

	query := NewQueryBuilder().
		Select(dbmodels.SelectScoringScoresColumns...).
		From(dbmodels.TABLE_SCORING_SCORES).
		Where(squirrel.Eq{
			"org_id":      entityRef.OrgId,
			"entity_type": entityRef.EntityType,
			"entity_id":   entityRef.EntityId,
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

	update := NewQueryBuilder().
		Update(dbmodels.TABLE_SCORING_SCORES).
		Set("deleted_at", squirrel.Expr("now()")).
		Where(squirrel.Eq{
			"org_id":      req.OrgId,
			"entity_type": req.EntityType,
			"entity_id":   req.EntityId,
		}).
		Where("deleted_at is null")

	if err := ExecBuilder(ctx, tx, update); err != nil {
		return models.ScoringScore{}, err
	}

	insert := NewQueryBuilder().
		Insert(dbmodels.TABLE_SCORING_SCORES).
		Columns(
			"id",
			"org_id",
			"entity_type",
			"entity_id",
			"score",
			"source",
			"overriden_by",
			"stale_at",
		).
		Values(
			uuid.Must(uuid.NewV7()),
			req.OrgId,
			req.EntityType,
			req.EntityId,
			req.Score,
			req.Source,
			req.OverridenBy,
			req.StaleAt,
		).
		Suffix("returning *")

	return SqlToModel(ctx, tx, insert, dbmodels.AdaptScoringScore)
}
