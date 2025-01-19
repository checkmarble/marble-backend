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
	if err := validateMarbleDbExecutor(exec); err != nil {
		return models.SanctionCheckConfig{}, err
	}

	sql := NewQueryBuilder().
		Select("*").From(dbmodels.TABLE_SANCTION_CHECK_CONFIGS).
		Where(squirrel.Eq{"scenario_iteration_id": scenarioIterationId})

	return SqlToModel(ctx, exec, sql, dbmodels.AdaptSanctionCheckConfig)
}

func (repo *MarbleDbRepository) UpdateSanctionCheckConfig(ctx context.Context, exec Executor,
	scenarioIterationId string, cfg models.UpdateSanctionCheckConfigInput,
) (models.SanctionCheckConfig, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return models.SanctionCheckConfig{}, err
	}

	sql := NewQueryBuilder().
		Insert(dbmodels.TABLE_SANCTION_CHECK_CONFIGS).
		Columns("scenario_iteration_id", "enabled", "forced_outcome", "score_modifier").
		Values(scenarioIterationId, cfg.Enabled,
			cfg.Outcome.ForceOutcome.MaybeString(),
			cfg.Outcome.ScoreModifier)

	updateFields := make([]string, 0, 4)

	if cfg.Enabled != nil {
		updateFields = append(updateFields, "enabled = EXCLUDED.enabled")
	}
	if cfg.Outcome.ForceOutcome != nil {
		switch *cfg.Outcome.ForceOutcome {
		case models.UnsetForcedOutcome:
			updateFields = append(updateFields, "forced_outcome = NULL")
		default:
			updateFields = append(updateFields, "forced_outcome = EXCLUDED.forced_outcome")
		}
	}
	if cfg.Outcome.ScoreModifier != nil {
		updateFields = append(updateFields, "score_modifier = EXCLUDED.score_modifier")
	}
	if len(updateFields) > 0 {
		updateFields = append(updateFields, "updated_at = NOW()")
	}

	switch len(updateFields) {
	case 0:
		sql = sql.Suffix("ON CONFLICT (scenario_iteration_id) DO NOTHING")
	default:
		sql = sql.Suffix(fmt.Sprintf("ON CONFLICT (scenario_iteration_id) DO UPDATE SET %s", strings.Join(updateFields, ",")))
	}

	sql = sql.Suffix(fmt.Sprintf("RETURNING %s",
		strings.Join(dbmodels.SanctionCheckConfigColumnList, ",")))

	return SqlToModel(ctx, exec, sql, dbmodels.AdaptSanctionCheckConfig)
}
