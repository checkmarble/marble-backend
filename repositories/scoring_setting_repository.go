package repositories

import (
	"context"

	"github.com/Masterminds/squirrel"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories/dbmodels"
	"github.com/google/uuid"
)

func (repo *MarbleDbRepository) GetScoringSettings(
	ctx context.Context,
	exec Executor,
	orgId uuid.UUID,
) (*models.ScoringSettings, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, err
	}

	query := NewQueryBuilder().
		Select(dbmodels.SelectScoringSettingsColumns...).
		From(dbmodels.TABLE_SCORING_SETTINGS).
		Where("org_id = ?", orgId)

	return SqlToOptionalModel(ctx, exec, query, dbmodels.AdaptScoringSetting)
}

func (repo *MarbleDbRepository) UpdateScoringSettings(
	ctx context.Context,
	exec Executor,
	settings models.ScoringSettings,
) (models.ScoringSettings, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return models.ScoringSettings{}, err
	}

	query := NewQueryBuilder().
		Insert(dbmodels.TABLE_SCORING_SETTINGS).
		Columns(
			"id",
			"org_id",
			"max_score",
			"created_at",
			"updated_at",
		).
		Values(
			uuid.Must(uuid.NewV7()),
			settings.OrgId,
			settings.MaxScore,
			squirrel.Expr("now()"),
			squirrel.Expr("now()"),
		).
		Suffix(`
			on conflict (org_id) do update set
				max_score = excluded.max_score,
				updated_at = now()
		`).
		Suffix("returning *")

	return SqlToModel(ctx, exec, query, dbmodels.AdaptScoringSetting)
}
