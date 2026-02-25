package repositories

import (
	"context"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories/dbmodels"
	"github.com/google/uuid"
)

func (repo *MarbleDbRepository) GetScoringSettings(
	ctx context.Context,
	exec Executor,
	orgId uuid.UUID,
) (*models.ScoringSetting, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, err
	}

	query := NewQueryBuilder().
		Select(dbmodels.SelectScoringSettingsColumns...).
		From(dbmodels.TABLE_SCORING_SETTINGS).
		Where("org_id = ?", orgId)

	return SqlToOptionalModel(ctx, exec, query, dbmodels.AdaptScoringSetting)
}
