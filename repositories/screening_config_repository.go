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
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/repositories/dbmodels"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/cockroachdb/errors"
)

const (
	DEFAULT_SCREENING_LIMIT = 10
)

func (repo *MarbleDbRepository) ListScreeningConfigs(
	ctx context.Context,
	exec Executor,
	scenarioIterationId string,
) ([]models.ScreeningConfig, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, err
	}

	sql := NewQueryBuilder().
		Select(dbmodels.ScreeningConfigColumnList...).
		From(dbmodels.TABLE_SCREENING_CONFIGS).
		Where(squirrel.Eq{"scenario_iteration_id": scenarioIterationId})

	return SqlToListOfModels(ctx, exec, sql, dbmodels.AdaptScreeningConfig)
}

func (repo *MarbleDbRepository) GetScreeningConfig(
	ctx context.Context,
	exec Executor,
	scenarioIterationId, screeningConfigId string,
) (models.ScreeningConfig, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return models.ScreeningConfig{}, err
	}

	sql := NewQueryBuilder().
		Select(dbmodels.ScreeningConfigColumnList...).
		From(dbmodels.TABLE_SCREENING_CONFIGS).
		Where(squirrel.Eq{"scenario_iteration_id": scenarioIterationId, "id": screeningConfigId})

	return SqlToModel(ctx, exec, sql, dbmodels.AdaptScreeningConfig)
}

func (repo *MarbleDbRepository) CreateScreeningConfig(ctx context.Context, exec Executor,
	scenarioIterationId string, cfg models.UpdateScreeningConfigInput,
) (models.ScreeningConfig, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return models.ScreeningConfig{}, err
	}

	var triggerRule *[]byte
	if cfg.TriggerRule != nil && cfg.TriggerRule.Function != ast.FUNC_UNDEFINED {
		astJson, err := dbmodels.SerializeFormulaAstExpression(cfg.TriggerRule)
		if err != nil {
			return models.ScreeningConfig{}, errors.Wrap(err,
				"could not serialize screening trigger rule")
		}

		triggerRule = astJson
	}

	var query map[string]dto.NodeDto

	if cfg.Query != nil {
		ser, err := pure_utils.MapValuesErr(cfg.Query, dto.AdaptNodeDto)
		if err != nil {
			return models.ScreeningConfig{}, err
		}
		query = ser
	}

	var counterpartyIdExpr *[]byte

	if cfg.CounterpartyIdExpression != nil {
		astJson, err := dbmodels.SerializeFormulaAstExpression(cfg.CounterpartyIdExpression)
		if err != nil {
			return models.ScreeningConfig{}, err
		}

		counterpartyIdExpr = astJson
	}

	forcedOutcome := models.BlockAndReview
	if cfg.ForcedOutcome != nil {
		forcedOutcome = *cfg.ForcedOutcome
	}

	configVersion := "v2"
	if cfg.ConfigVersion != "" {
		configVersion = cfg.ConfigVersion
	}

	sql := NewQueryBuilder().
		Insert(dbmodels.TABLE_SCREENING_CONFIGS).
		Columns(
			"stable_id",
			"scenario_iteration_id",
			"name",
			"description",
			"rule_group",
			"datasets",
			"threshold",
			"forced_outcome",
			"trigger_rule",
			"entity_type",
			"query",
			"counterparty_id_expression",
			"preprocessing",
			"config_version").
		Values(
			squirrel.Expr("coalesce(?, gen_random_uuid())", cfg.StableId),
			scenarioIterationId,
			cfg.Name,
			utils.Or(cfg.Description, ""),
			utils.Or(cfg.RuleGroup, ""),
			cfg.Datasets,
			cfg.Threshold,
			forcedOutcome.String(),
			triggerRule,
			utils.Or(cfg.EntityType, "Thing"),
			query,
			counterpartyIdExpr,
			utils.Or(cfg.Preprocessing, models.ScreeningConfigPreprocessing{}),
			configVersion,
		).
		Suffix(fmt.Sprintf("RETURNING %s", strings.Join(dbmodels.ScreeningConfigColumnList, ",")))

	return SqlToModel(ctx, exec, sql, dbmodels.AdaptScreeningConfig)
}

func (repo *MarbleDbRepository) UpdateScreeningConfig(ctx context.Context, exec Executor,
	scenarioIterationId string, id string, cfg models.UpdateScreeningConfigInput,
) (models.ScreeningConfig, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return models.ScreeningConfig{}, err
	}

	var triggerRule *[]byte
	if cfg.TriggerRule != nil && cfg.TriggerRule.Function != ast.FUNC_UNDEFINED {
		astJson, err := dbmodels.SerializeFormulaAstExpression(cfg.TriggerRule)
		if err != nil {
			return models.ScreeningConfig{}, errors.Wrap(err,
				"could not serialize screening trigger rule")
		}

		triggerRule = astJson
	}

	var query map[string]dto.NodeDto

	if cfg.Query != nil {
		ser, err := pure_utils.MapValuesErr(cfg.Query, dto.AdaptNodeDto)
		if err != nil {
			return models.ScreeningConfig{}, err
		}
		query = ser
	}

	var counterpartyIdExpr *[]byte

	if cfg.CounterpartyIdExpression != nil {
		astJson, err := dbmodels.SerializeFormulaAstExpression(cfg.CounterpartyIdExpression)
		if err != nil {
			return models.ScreeningConfig{}, err
		}

		counterpartyIdExpr = astJson
	}

	forcedOutcome := models.BlockAndReview
	if cfg.ForcedOutcome != nil {
		forcedOutcome = *cfg.ForcedOutcome
	}

	sql := NewQueryBuilder().
		Update(dbmodels.TABLE_SCREENING_CONFIGS).
		Where(squirrel.Eq{"id": id}).
		Suffix(fmt.Sprintf("RETURNING %s", strings.Join(dbmodels.ScreeningConfigColumnList, ","))).
		Set("updated_at", time.Now())

	updateFields := false

	if cfg.Name != nil {
		sql = sql.Set("name", cfg.Name)
		updateFields = true
	}
	if cfg.Description != nil {
		sql = sql.Set("description", cfg.Description)
		updateFields = true
	}
	if cfg.RuleGroup != nil {
		sql = sql.Set("rule_group", cfg.RuleGroup)
		updateFields = true
	}
	if cfg.Datasets != nil {
		sql = sql.Set("datasets", cfg.Datasets)
		updateFields = true
	}
	if cfg.Threshold != nil {
		switch *cfg.Threshold {
		case 0:
			sql = sql.Set("threshold", nil)
		default:
			sql = sql.Set("threshold", cfg.Threshold)
		}
		updateFields = true
	}
	if cfg.TriggerRule != nil {
		switch cfg.TriggerRule.Function {
		case ast.FUNC_UNDEFINED:
			sql = sql.Set("trigger_rule", nil)
		default:
			sql = sql.Set("trigger_rule", triggerRule)
		}
		updateFields = true
	}
	if cfg.Query != nil {
		sql = sql.Set("entity_type", cfg.EntityType)
		updateFields = true
	}
	if cfg.Query != nil {
		sql = sql.Set("query", query)
		updateFields = true
	}
	if cfg.CounterpartyIdExpression != nil {
		switch cfg.CounterpartyIdExpression.Function {
		case ast.FUNC_UNDEFINED:
			sql = sql.Set("counterparty_id_expression", nil)
		default:
			sql = sql.Set("counterparty_id_expression", counterpartyIdExpr)
		}
		updateFields = true
	}
	if cfg.ForcedOutcome != nil {
		sql = sql.Set("forced_outcome", forcedOutcome)
		updateFields = true
	}
	if cfg.Preprocessing != nil {
		sql = sql.Set("preprocessing", *cfg.Preprocessing)
		updateFields = true
	}

	if !updateFields {
		return repo.GetScreeningConfig(ctx, exec, scenarioIterationId, id)
	}

	return SqlToModel(ctx, exec, sql, dbmodels.AdaptScreeningConfig)
}

func (repo *MarbleDbRepository) DeleteScreeningConfig(ctx context.Context, exec Executor, scenarioIterationId, configId string) error {
	sql := NewQueryBuilder().
		Delete(dbmodels.TABLE_SCREENING_CONFIGS).
		Where(squirrel.Eq{
			"scenario_iteration_id": scenarioIterationId,
			"id":                    configId,
		})

	if err := ExecBuilder(ctx, exec, sql); err != nil {
		return err
	}

	return nil
}
