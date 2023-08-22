package repositories

import (
	"fmt"
	"marble/marble-backend/models"
	"marble/marble-backend/repositories/dbmodels"
	"marble/marble-backend/utils"

	"github.com/Masterminds/squirrel"
)

type ScenarioIterationWriteRepository interface {
	CreateScenarioIteration(tx Transaction, organizationId string, scenarioIteration models.CreateScenarioIterationInput) (models.ScenarioIteration, error)
	UpdateScenarioIteration(tx Transaction, organizationId string, scenarioIteration models.UpdateScenarioIterationInput) (models.ScenarioIteration, error)
	DeleteScenarioIteration(tx Transaction, scenarioIterationId string) error
}

type ScenarioIterationWriteRepositoryPostgresql struct {
	transactionFactory TransactionFactory
	ruleRepository     RuleRepository
}

func (repo *ScenarioIterationWriteRepositoryPostgresql) DeleteScenarioIteration(tx Transaction, scenarioIterationId string) error {
	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(tx)

	_, err := pgTx.ExecBuilder(NewQueryBuilder().Delete(dbmodels.TABLE_SCENARIO_ITERATIONS).Where("id = ?", scenarioIterationId))
	return err
}

func (repo *ScenarioIterationWriteRepositoryPostgresql) CreateScenarioIteration(tx Transaction, organizationId string, scenarioIteration models.CreateScenarioIterationInput) (models.ScenarioIteration, error) {
	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(tx)

	query := NewQueryBuilder().Insert(dbmodels.TABLE_SCENARIO_ITERATIONS).
		Columns(
			"id",
			"org_id",
			"scenario_id",
		).Values(
		utils.NewPrimaryKey(organizationId),
		organizationId,
		scenarioIteration.ScenarioId,
	).Suffix("RETURNING *")

	if scenarioIteration.Body != nil {
		triggerCondition, err := dbmodels.SerializeFormulaAstExpression(scenarioIteration.Body.TriggerConditionAstExpression)
		if err != nil {
			return models.ScenarioIteration{}, fmt.Errorf("unable to marshal trigger condition ast expression: %w", err)
		}
		query.Columns(
			"score_review_threshold",
			"score_reject_threshold",
			"trigger_condition_ast_expression",
			"batch_trigger_sql",
			"schedule",
		).Values(
			scenarioIteration.Body.ScoreReviewThreshold,
			scenarioIteration.Body.ScoreRejectThreshold,
			triggerCondition,
			scenarioIteration.Body.BatchTriggerSQL,
			scenarioIteration.Body.Schedule,
		)
	}

	createdIteration, err := SqlToModelAdapterWithErr(
		pgTx,
		query,
		dbmodels.AdaptScenarioIteration,
	)
	if err != nil {
		return models.ScenarioIteration{}, err
	}

	if scenarioIteration.Body != nil {
		createdRules, err := repo.ruleRepository.CreateRules(tx, scenarioIteration.Body.Rules)
		if err != nil {
			return models.ScenarioIteration{}, fmt.Errorf("unable to create scenario iteration rules: %w", err)
		}
		createdIteration.Rules = createdRules
	}

	return createdIteration, nil
}

func (repo *ScenarioIterationWriteRepositoryPostgresql) UpdateScenarioIteration(tx Transaction, organizationId string, scenarioIteration models.UpdateScenarioIterationInput) (models.ScenarioIteration, error) {
	if scenarioIteration.Body == nil {
		return models.ScenarioIteration{}, fmt.Errorf("nothing to update")
	}

	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(tx)

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
	updatedIteration, err := SqlToModelAdapterWithErr(pgTx, sql, dbmodels.AdaptScenarioIteration)
	if err != nil {
		return models.ScenarioIteration{}, err
	}
	return updatedIteration, nil
}
