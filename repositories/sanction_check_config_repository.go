package repositories

import (
	"context"
	"fmt"
	"strings"

	"github.com/Masterminds/squirrel"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories/dbmodels"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/cockroachdb/errors"
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

	var triggerRule *[]byte
	if cfg.TriggerRule != nil {
		astJson, err := dbmodels.SerializeFormulaAstExpression(cfg.TriggerRule)
		if err != nil {
			return models.SanctionCheckConfig{}, errors.Wrap(err,
				"could not serialize sanction check trigger rule")
		}

		triggerRule = astJson
	}

	sql := NewQueryBuilder().
		Insert(dbmodels.TABLE_SANCTION_CHECK_CONFIGS).
		Columns("scenario_iteration_id", "enabled", "forced_outcome", "score_modifier", "trigger_rule").
		Values(
			scenarioIterationId,
			utils.Or(cfg.Enabled, true),
			cfg.Outcome.ForceOutcome.MaybeString(),
			utils.Or(cfg.Outcome.ScoreModifier, 0),
			utils.Or(triggerRule, []byte(``)),
		)

	updateFields := make([]string, 0, 4)

	if cfg.Enabled != nil {
		updateFields = append(updateFields, "enabled = EXCLUDED.enabled")
	}
	if cfg.TriggerRule != nil {
		updateFields = append(updateFields, "trigger_rule = EXCLUDED.trigger_rule")
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
