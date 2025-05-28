package repositories

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Masterminds/squirrel"
	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/models/ast"
	"github.com/checkmarble/marble-backend/repositories/dbmodels"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/cockroachdb/errors"
)

func (repo *MarbleDbRepository) ListSanctionCheckConfigs(
	ctx context.Context,
	exec Executor,
	scenarioIterationId string,
) ([]models.SanctionCheckConfig, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, err
	}

	sql := NewQueryBuilder().
		Select(dbmodels.SanctionCheckConfigColumnList...).
		From(dbmodels.TABLE_SANCTION_CHECK_CONFIGS).
		Where(squirrel.Eq{"scenario_iteration_id": scenarioIterationId})

	return SqlToListOfModels(ctx, exec, sql, dbmodels.AdaptSanctionCheckConfig)
}

func (repo *MarbleDbRepository) GetSanctionCheckConfig(
	ctx context.Context,
	exec Executor,
	scenarioIterationId, sanctionCheckConfigId string,
) (*models.SanctionCheckConfig, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, err
	}

	sql := NewQueryBuilder().
		Select(dbmodels.SanctionCheckConfigColumnList...).
		From(dbmodels.TABLE_SANCTION_CHECK_CONFIGS).
		Where(squirrel.Eq{"scenario_iteration_id": scenarioIterationId, "id": sanctionCheckConfigId})

	return SqlToOptionalModel(ctx, exec, sql, dbmodels.AdaptSanctionCheckConfig)
}

func (repo *MarbleDbRepository) CreateSanctionCheckConfig(ctx context.Context, exec Executor,
	scenarioIterationId string, cfg models.UpdateSanctionCheckConfigInput,
) (models.SanctionCheckConfig, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return models.SanctionCheckConfig{}, err
	}

	var triggerRule *[]byte
	if cfg.TriggerRule != nil && cfg.TriggerRule.Function != ast.FUNC_UNDEFINED {
		astJson, err := dbmodels.SerializeFormulaAstExpression(cfg.TriggerRule)
		if err != nil {
			return models.SanctionCheckConfig{}, errors.Wrap(err,
				"could not serialize sanction check trigger rule")
		}

		triggerRule = astJson
	}

	var query *dbmodels.DBSanctionCheckConfigQueryInput

	if cfg.Query != nil {
		query = &dbmodels.DBSanctionCheckConfigQueryInput{}

		if cfg.Query.Name != nil {
			ser, err := dto.AdaptNodeDto(*cfg.Query.Name)
			if err != nil {
				return models.SanctionCheckConfig{}, err
			}
			query.Name = &ser
		}

		if cfg.Query.Label != nil {
			ser, err := dto.AdaptNodeDto(*cfg.Query.Label)
			if err != nil {
				return models.SanctionCheckConfig{}, err
			}
			query.Label = &ser
		}
	}

	var counterpartyIdExpr *[]byte

	if cfg.CounterpartyIdExpression != nil {
		astJson, err := dbmodels.SerializeFormulaAstExpression(cfg.CounterpartyIdExpression)
		if err != nil {
			return models.SanctionCheckConfig{}, err
		}

		counterpartyIdExpr = astJson
	}

	forcedOutcome := models.BlockAndReview
	if cfg.ForcedOutcome != nil {
		forcedOutcome = *cfg.ForcedOutcome
	}

	sql := NewQueryBuilder().
		Insert(dbmodels.TABLE_SANCTION_CHECK_CONFIGS).
		Columns(
			"stable_id",
			"scenario_iteration_id",
			"name",
			"description",
			"rule_group",
			"datasets",
			"forced_outcome",
			"trigger_rule",
			"query",
			"counterparty_id_expression",
			"config_version").
		Values(
			squirrel.Expr("coalesce(?, gen_random_uuid())", cfg.StableId),
			scenarioIterationId,
			cfg.Name,
			utils.Or(cfg.Description, ""),
			utils.Or(cfg.RuleGroup, ""),
			cfg.Datasets,
			forcedOutcome.String(),
			triggerRule,
			query,
			counterpartyIdExpr,
			"v2",
		).
		Suffix(fmt.Sprintf("RETURNING %s", strings.Join(dbmodels.SanctionCheckConfigColumnList, ",")))

	return SqlToModel(ctx, exec, sql, dbmodels.AdaptSanctionCheckConfig)
}

func (repo *MarbleDbRepository) UpdateSanctionCheckConfig(ctx context.Context, exec Executor,
	scenarioIterationId string, id string, cfg models.UpdateSanctionCheckConfigInput,
) (models.SanctionCheckConfig, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return models.SanctionCheckConfig{}, err
	}

	var triggerRule *[]byte
	if cfg.TriggerRule != nil && cfg.TriggerRule.Function != ast.FUNC_UNDEFINED {
		astJson, err := dbmodels.SerializeFormulaAstExpression(cfg.TriggerRule)
		if err != nil {
			return models.SanctionCheckConfig{}, errors.Wrap(err,
				"could not serialize sanction check trigger rule")
		}

		triggerRule = astJson
	}

	var query *dbmodels.DBSanctionCheckConfigQueryInput

	if cfg.Query != nil {
		query = &dbmodels.DBSanctionCheckConfigQueryInput{}

		if cfg.Query.Name != nil {
			ser, err := dto.AdaptNodeDto(*cfg.Query.Name)
			if err != nil {
				return models.SanctionCheckConfig{}, err
			}
			query.Name = &ser
		}

		if cfg.Query.Label != nil {
			ser, err := dto.AdaptNodeDto(*cfg.Query.Label)
			if err != nil {
				return models.SanctionCheckConfig{}, err
			}
			query.Label = &ser
		}
	}

	var counterpartyIdExpr *[]byte

	if cfg.CounterpartyIdExpression != nil {
		astJson, err := dbmodels.SerializeFormulaAstExpression(cfg.CounterpartyIdExpression)
		if err != nil {
			return models.SanctionCheckConfig{}, err
		}

		counterpartyIdExpr = astJson
	}

	forcedOutcome := models.BlockAndReview
	if cfg.ForcedOutcome != nil {
		forcedOutcome = *cfg.ForcedOutcome
	}

	sql := NewQueryBuilder().
		Update(dbmodels.TABLE_SANCTION_CHECK_CONFIGS).
		Where(squirrel.Eq{"id": id}).
		Suffix(fmt.Sprintf("RETURNING %s", strings.Join(dbmodels.SanctionCheckConfigColumnList, ",")))

	updateFields := make([]string, 0, 4)

	if cfg.Name != nil {
		sql = sql.Set("name", cfg.Name)
	}
	if cfg.Description != nil {
		sql = sql.Set("description", cfg.Description)
	}
	if cfg.RuleGroup != nil {
		sql = sql.Set("rule_group", cfg.RuleGroup)
	}
	if cfg.Datasets != nil {
		sql = sql.Set("datasets", cfg.Datasets)
	}
	if cfg.TriggerRule != nil {
		sql = sql.Set("trigger_rule", triggerRule)
	}
	if cfg.Query != nil {
		sql = sql.Set("query", query)
	}
	if cfg.CounterpartyIdExpression != nil {
		sql = sql.Set("counterparty_id_expression", counterpartyIdExpr)
	}
	if cfg.ForcedOutcome != nil {
		sql = sql.Set("forced_outcome", forcedOutcome)
	}
	if len(updateFields) > 0 {
		sql = sql.Set("updated_at", time.Now())
	}

	return SqlToModel(ctx, exec, sql, dbmodels.AdaptSanctionCheckConfig)
}

func (repo *MarbleDbRepository) DeleteSanctionCheckConfig(ctx context.Context, exec Executor, scenarioIterationId string) error {
	sql := NewQueryBuilder().
		Delete(dbmodels.TABLE_SANCTION_CHECK_CONFIGS).
		Where(squirrel.Eq{"scenario_iteration_id": scenarioIterationId})

	if err := ExecBuilder(ctx, exec, sql); err != nil {
		return err
	}

	return nil
}
