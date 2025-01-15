package repositories

import (
	"context"
	"fmt"
	"strings"

	"github.com/Masterminds/squirrel"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories/dbmodels"
)

func (repo *MarbleDbRepository) GetSanctionCheckConfig(ctx context.Context, exec Executor,
	scenarioIterationId string,
) (models.SanctionCheckConfig, error) {
	sql := NewQueryBuilder().
		Select("*").From(dbmodels.TABLE_SANCTION_CHECK_CONFIGS).
		Where(squirrel.Eq{"scenario_iteration_id": scenarioIterationId})

	return SqlToModel(ctx, exec, sql, dbmodels.AdaptSanctionCheckConfig)
}

func (repo *MarbleDbRepository) UpdateSanctionCheckConfig(ctx context.Context, exec Executor,
	scenarioIterationId string, sanctionCheckConfig models.SanctionCheckConfig,
) (models.SanctionCheckConfig, error) {
	sql := NewQueryBuilder().
		Insert(dbmodels.TABLE_SANCTION_CHECK_CONFIGS).
		Columns("scenario_iteration_id", "enabled").
		Values(scenarioIterationId, sanctionCheckConfig.Enabled).
		Suffix("ON CONFLICT (scenario_iteration_id) DO UPDATE").
		Suffix("SET enabled = EXCLUDED.enabled, updated_at = NOW()").
		Suffix(fmt.Sprintf("RETURNING %s", strings.Join(dbmodels.SanctionCheckConfigColumnList, ",")))

	return SqlToModel(ctx, exec, sql, dbmodels.AdaptSanctionCheckConfig)
}
