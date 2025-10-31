package repositories

import (
	"context"

	"github.com/Masterminds/squirrel"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories/dbmodels"
	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
)

func (repo *MarbleDbRepository) GetScreeningMonitoringConfig(ctx context.Context, exec Executor, Id uuid.UUID) (models.ScreeningMonitoringConfig, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return models.ScreeningMonitoringConfig{}, err
	}

	sql := NewQueryBuilder().
		Select(dbmodels.ScreeningMonitoringConfigColumnList...).
		From(dbmodels.TABLE_SCREENING_MONITORING_CONFIGS).
		Where(squirrel.Eq{"id": Id})

	return SqlToModel(ctx, exec, sql, dbmodels.AdaptScreeningMonitoringConfig)
}

func (repo *MarbleDbRepository) GetScreeningMonitoringConfigsByOrgId(
	ctx context.Context,
	exec Executor,
	orgId string,
) ([]models.ScreeningMonitoringConfig, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, err
	}

	sql := NewQueryBuilder().
		Select(dbmodels.ScreeningMonitoringConfigColumnList...).
		From(dbmodels.TABLE_SCREENING_MONITORING_CONFIGS).
		Where(squirrel.Eq{"org_id": orgId})

	return SqlToListOfModels(ctx, exec, sql, dbmodels.AdaptScreeningMonitoringConfig)
}

// `enabled` is set to true by default (see: `20251028145610_screening_monitoring_config.sql` migration)
func (repo *MarbleDbRepository) CreateScreeningMonitoringConfig(ctx context.Context, exec Executor,
	input models.CreateScreeningMonitoringConfig,
) (models.ScreeningMonitoringConfig, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return models.ScreeningMonitoringConfig{}, err
	}

	if len(input.Datasets) == 0 {
		return models.ScreeningMonitoringConfig{},
			errors.New("datasets are required for screening monitoring config")
	}

	sql := NewQueryBuilder().
		Insert(dbmodels.TABLE_SCREENING_MONITORING_CONFIGS).
		Suffix("RETURNING *").
		Columns(
			"org_id",
			"name",
			"description",
			"datasets",
			"match_threshold",
			"match_limit",
		).
		Values(
			input.OrgId,
			input.Name,
			input.Description,
			input.Datasets,
			input.MatchThreshold,
			input.MatchLimit,
		)

	return SqlToModel(ctx, exec, sql, dbmodels.AdaptScreeningMonitoringConfig)
}

func (repo *MarbleDbRepository) UpdateScreeningMonitoringConfig(
	ctx context.Context,
	exec Executor,
	id uuid.UUID,
	input models.UpdateScreeningMonitoringConfig,
) (models.ScreeningMonitoringConfig, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return models.ScreeningMonitoringConfig{}, err
	}

	countUpdate := 0

	sql := NewQueryBuilder().
		Update(dbmodels.TABLE_SCREENING_MONITORING_CONFIGS).
		Suffix("RETURNING *").
		Set("updated_at", squirrel.Expr("NOW()")).
		Where(squirrel.Eq{"id": id})

	if input.Name != nil {
		sql = sql.Set("name", *input.Name)
		countUpdate++
	}
	if input.Description != nil {
		sql = sql.Set("description", *input.Description)
		countUpdate++
	}
	if input.Datasets != nil {
		sql = sql.Set("datasets", *input.Datasets)
		countUpdate++
	}
	if input.MatchThreshold != nil {
		sql = sql.Set("match_threshold", *input.MatchThreshold)
		countUpdate++
	}
	if input.MatchLimit != nil {
		sql = sql.Set("match_limit", *input.MatchLimit)
		countUpdate++
	}
	if input.Enabled != nil {
		sql = sql.Set("enabled", *input.Enabled)
		countUpdate++
	}

	if countUpdate == 0 {
		config, err := repo.GetScreeningMonitoringConfig(ctx, exec, id)
		if err != nil {
			return models.ScreeningMonitoringConfig{}, err
		}
		return config, nil
	}

	return SqlToModel(ctx, exec, sql, dbmodels.AdaptScreeningMonitoringConfig)
}
