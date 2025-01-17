package repositories

import (
	"context"
	"fmt"
	"strings"

	"github.com/Masterminds/squirrel"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories/dbmodels"
	"github.com/checkmarble/marble-backend/utils"
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
	scenarioIterationId string, sanctionCheckConfig models.UpdateSanctionCheckConfigInput,
) (models.SanctionCheckConfig, error) {
	var outcome *string

	if sanctionCheckConfig.Outcome.ForceOutcome != nil &&
		*sanctionCheckConfig.Outcome.ForceOutcome != models.Approve {
		outcome = utils.Ptr(sanctionCheckConfig.Outcome.ForceOutcome.String())
	}

	sql := NewQueryBuilder().
		Insert(dbmodels.TABLE_SANCTION_CHECK_CONFIGS).
		Columns("scenario_iteration_id", "enabled", "forced_outcome", "score_modifier").
		Values(scenarioIterationId, sanctionCheckConfig.Enabled,
			outcome,
			sanctionCheckConfig.Outcome.ScoreModifier).
		Suffix("ON CONFLICT (scenario_iteration_id) DO UPDATE SET")

	updateFields := make([]string, 0, 4)

	if sanctionCheckConfig.Enabled != nil {
		updateFields = append(updateFields, "enabled = EXCLUDED.enabled")
	}
	if sanctionCheckConfig.Outcome.ForceOutcome != nil {
		updateFields = append(updateFields, "forced_outcome = EXCLUDED.forced_outcome")
	}
	if sanctionCheckConfig.Outcome.ScoreModifier != nil {
		updateFields = append(updateFields, "score_modifier = EXCLUDED.score_modifier")
	}

	updateFields = append(updateFields, "updated_at = NOW()")

	sql = sql.Suffix(strings.Join(updateFields, ","))

	sql = sql.Suffix(fmt.Sprintf("RETURNING %s",
		strings.Join(dbmodels.SanctionCheckConfigColumnList, ",")))

	return SqlToModel(ctx, exec, sql, dbmodels.AdaptSanctionCheckConfig)
}
