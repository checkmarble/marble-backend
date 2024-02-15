package repositories

import (
	"context"
	"fmt"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories/dbmodels"
	"github.com/checkmarble/marble-backend/utils"

	"github.com/Masterminds/squirrel"
)

func (repo *MarbleDbRepository) DeleteScenarioIteration(ctx context.Context, exec Executor, scenarioIterationId string) error {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return err
	}

	err := ExecBuilder(ctx, exec, NewQueryBuilder().Delete(dbmodels.TABLE_SCENARIO_ITERATIONS).Where("id = ?", scenarioIterationId))
	return err
}

func (repo *MarbleDbRepository) CreateScenarioIterationAndRules(ctx context.Context, exec Executor, organizationId string, scenarioIteration models.CreateScenarioIterationInput) (models.ScenarioIteration, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return models.ScenarioIteration{}, err
	}

	query := NewQueryBuilder().Insert(dbmodels.TABLE_SCENARIO_ITERATIONS).
		Columns(
			"id",
			"org_id",
			"scenario_id",
		).Suffix("RETURNING *")

	scenarioIterationBodyInput := scenarioIteration.Body
	if scenarioIterationBodyInput != nil {

		var triggerCondition *[]byte
		if scenarioIterationBodyInput != nil && scenarioIterationBodyInput.TriggerConditionAstExpression != nil {
			var err error
			triggerCondition, err = dbmodels.SerializeFormulaAstExpression(scenarioIterationBodyInput.TriggerConditionAstExpression)
			if err != nil {
				return models.ScenarioIteration{}, fmt.Errorf("unable to marshal trigger condition ast expression: %w", err)
			}
		}
		query = query.Columns(
			"score_review_threshold",
			"score_reject_threshold",
			"trigger_condition_ast_expression",
			"batch_trigger_sql",
			"schedule",
		).Values(
			utils.NewPrimaryKey(organizationId),
			organizationId,
			scenarioIteration.ScenarioId,
			scenarioIterationBodyInput.ScoreReviewThreshold,
			scenarioIterationBodyInput.ScoreRejectThreshold,
			triggerCondition,
			scenarioIterationBodyInput.BatchTriggerSQL,
			scenarioIterationBodyInput.Schedule,
		)
	} else {
		query = query.Values(
			utils.NewPrimaryKey(organizationId),
			organizationId,
			scenarioIteration.ScenarioId,
		)
	}

	createdIteration, err := SqlToModel(
		ctx,
		exec,
		query,
		dbmodels.AdaptScenarioIteration,
	)
	if err != nil {
		return models.ScenarioIteration{}, err
	}

	if scenarioIteration.Body != nil && len(scenarioIteration.Body.Rules) > 0 {
		for i := range scenarioIteration.Body.Rules {
			scenarioIteration.Body.Rules[i].Id = utils.NewPrimaryKey(organizationId)
			scenarioIteration.Body.Rules[i].OrganizationId = organizationId
			scenarioIteration.Body.Rules[i].ScenarioIterationId = createdIteration.Id
		}
		createdRules, err := repo.CreateRules(ctx, exec, scenarioIteration.Body.Rules)
		if err != nil {
			return models.ScenarioIteration{}, fmt.Errorf("unable to create scenario iteration rules: %w", err)
		}
		createdIteration.Rules = createdRules
	}

	return createdIteration, nil
}

func (repo *MarbleDbRepository) UpdateScenarioIteration(ctx context.Context, exec Executor, scenarioIteration models.UpdateScenarioIterationInput) (models.ScenarioIteration, error) {
	if scenarioIteration.Body == nil {
		return models.ScenarioIteration{}, fmt.Errorf("nothing to update")
	}

	if err := validateMarbleDbExecutor(exec); err != nil {
		return models.ScenarioIteration{}, err
	}

	sql := NewQueryBuilder().
		Update(dbmodels.TABLE_SCENARIO_ITERATIONS).
		Set("updated_at", squirrel.Expr("NOW()")).
		Where("id = ?", scenarioIteration.Id).
		Suffix("RETURNING *")
	if scenarioIteration.Body.ScoreReviewThreshold != nil {
		sql = sql.Set("score_review_threshold", scenarioIteration.Body.ScoreReviewThreshold)
	}
	if scenarioIteration.Body.ScoreRejectThreshold != nil {
		sql = sql.Set("score_reject_threshold", scenarioIteration.Body.ScoreRejectThreshold)
	}
	if scenarioIteration.Body.Schedule != nil {
		sql = sql.Set("schedule", scenarioIteration.Body.Schedule)
	}
	if scenarioIteration.Body.BatchTriggerSQL != nil {
		sql = sql.Set("batch_trigger_sql", scenarioIteration.Body.BatchTriggerSQL)
	}
	if scenarioIteration.Body.TriggerConditionAstExpression != nil {
		triggerCondition, err := dbmodels.SerializeFormulaAstExpression(scenarioIteration.Body.TriggerConditionAstExpression)
		if err != nil {
			return models.ScenarioIteration{}, fmt.Errorf("unable to marshal trigger condition ast expression: %w", err)
		}
		sql = sql.Set("trigger_condition_ast_expression", triggerCondition)
	}
	updatedIteration, err := SqlToModel(ctx, exec, sql, dbmodels.AdaptScenarioIteration)
	if err != nil {
		return models.ScenarioIteration{}, err
	}
	return updatedIteration, nil
}

func (repo *MarbleDbRepository) UpdateScenarioIterationVersion(ctx context.Context, exec Executor, scenarioIterationId string, newVersion int) error {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return err
	}

	err := ExecBuilder(
		ctx,
		exec,
		NewQueryBuilder().Update(dbmodels.TABLE_SCENARIO_ITERATIONS).
			Set("version", newVersion).
			Where(squirrel.Eq{"id": scenarioIterationId}),
	)
	return err
}
