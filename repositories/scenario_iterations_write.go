package repositories

import (
	"context"
	"fmt"
	"strings"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/models/ast"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/repositories/dbmodels"

	"github.com/Masterminds/squirrel"
)

func (repo *MarbleDbRepository) DeleteScenarioIteration(ctx context.Context, exec Executor, scenarioIterationId string) error {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return err
	}

	err := ExecBuilder(ctx, exec, NewQueryBuilder().Delete(dbmodels.TABLE_SCENARIO_ITERATIONS).Where("id = ?", scenarioIterationId))
	return err
}

func (repo *MarbleDbRepository) CreateScenarioIterationAndRules(ctx context.Context, exec Executor,
	organizationId string, scenarioIteration models.CreateScenarioIterationInput,
) (models.ScenarioIteration, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return models.ScenarioIteration{}, err
	}

	query := NewQueryBuilder().Insert(dbmodels.TABLE_SCENARIO_ITERATIONS).
		Columns(
			"id",
			"org_id",
			"scenario_id",
		).Suffix(fmt.Sprintf("RETURNING %s", strings.Join(dbmodels.SelectScenarioIterationColumn, ",")))

	scenarioIterationBodyInput := scenarioIteration.Body
	if scenarioIterationBodyInput != nil {

		var triggerCondition *[]byte
		if scenarioIterationBodyInput.TriggerConditionAstExpression != nil {
			var err error
			triggerCondition, err = dbmodels.SerializeFormulaAstExpression(
				scenarioIterationBodyInput.TriggerConditionAstExpression)
			if err != nil {
				return models.ScenarioIteration{}, fmt.Errorf(
					"unable to marshal trigger condition ast expression: %w", err)
			}
		}
		query = query.Columns(
			"score_review_threshold",
			"score_block_and_review_threshold",
			"score_reject_threshold",
			"trigger_condition_ast_expression",
			"schedule",
		).Values(
			pure_utils.NewPrimaryKey(organizationId),
			organizationId,
			scenarioIteration.ScenarioId,
			scenarioIterationBodyInput.ScoreReviewThreshold,
			scenarioIterationBodyInput.ScoreBlockAndReviewThreshold,
			scenarioIterationBodyInput.ScoreDeclineThreshold,
			triggerCondition,
			scenarioIterationBodyInput.Schedule,
		)
	} else {
		query = query.Values(
			pure_utils.NewPrimaryKey(organizationId),
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
			scenarioIteration.Body.Rules[i].Id = pure_utils.NewPrimaryKey(organizationId)
			scenarioIteration.Body.Rules[i].OrganizationId = organizationId
			scenarioIteration.Body.Rules[i].ScenarioIterationId = createdIteration.Id
		}
		createdRules, err := repo.CreateRules(ctx, exec, scenarioIteration.Body.Rules)
		if err != nil {
			return models.ScenarioIteration{}, fmt.Errorf(
				"unable to create scenario iteration rules: %w", err)
		}
		createdIteration.Rules = createdRules
	}

	return createdIteration, nil
}

func (repo *MarbleDbRepository) UpdateScenarioIteration(ctx context.Context, exec Executor,
	scenarioIteration models.UpdateScenarioIterationInput,
) (models.ScenarioIteration, error) {
	countUpdate := 0

	if err := validateMarbleDbExecutor(exec); err != nil {
		return models.ScenarioIteration{}, err
	}

	sql := NewQueryBuilder().
		Update(dbmodels.TABLE_SCENARIO_ITERATIONS).
		Set("updated_at", squirrel.Expr("NOW()")).
		Where("id = ?", scenarioIteration.Id).
		Suffix(fmt.Sprintf("RETURNING %s", strings.Join(dbmodels.SelectScenarioIterationColumn, ",")))
	if scenarioIteration.Body.ScoreReviewThreshold != nil {
		sql = sql.Set("score_review_threshold", scenarioIteration.Body.ScoreReviewThreshold)
		countUpdate++
	}
	if scenarioIteration.Body.ScoreBlockAndReviewThreshold != nil {
		sql = sql.Set("score_block_and_review_threshold",
			scenarioIteration.Body.ScoreBlockAndReviewThreshold)
		countUpdate++
	}
	if scenarioIteration.Body.ScoreDeclineThreshold != nil {
		sql = sql.Set("score_reject_threshold", scenarioIteration.Body.ScoreDeclineThreshold)
		countUpdate++
	}
	if scenarioIteration.Body.Schedule != nil {
		sql = sql.Set("schedule", scenarioIteration.Body.Schedule)
		countUpdate++
	}
	if scenarioIteration.Body.TriggerConditionAstExpression != nil {
		if scenarioIteration.Body.TriggerConditionAstExpression.Function != ast.FUNC_UNDEFINED {
			triggerCondition, err := dbmodels.SerializeFormulaAstExpression(
				scenarioIteration.Body.TriggerConditionAstExpression)
			if err != nil {
				return models.ScenarioIteration{}, fmt.Errorf(
					"unable to marshal trigger condition ast expression: %w", err)
			}
			sql = sql.Set("trigger_condition_ast_expression", triggerCondition)
		} else {
			sql = sql.Set("trigger_condition_ast_expression", nil)
		}
		countUpdate++
	}
	if countUpdate == 0 {
		return repo.GetScenarioIteration(ctx, exec, scenarioIteration.Id)
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
