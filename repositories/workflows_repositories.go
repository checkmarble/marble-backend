package repositories

import (
	"context"

	"github.com/Masterminds/squirrel"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories/dbmodels"
	"github.com/google/uuid"
)

func (repo *MarbleDbRepository) ListWorkflowsForScenario(ctx context.Context, exec Executor, scenarioId string) ([]models.Workflow, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, err
	}

	sql := NewQueryBuilder().
		Select(columnsNames("r", dbmodels.WorkflowRuleColumns)...).
		Column("array_agg(row(c.*)) filter (where c.id is not null) as conditions").
		Column("array_agg(row(a.*)) filter (where a.id is not null) as actions").
		From(dbmodels.TABLE_WORKFLOW_RULES + " r").
		LeftJoin(dbmodels.TABLE_WORKFLOW_CONDITIONS + " c on c.rule_id = r.id").
		LeftJoin(dbmodels.TABLE_WORKFLOW_ACTIONS + " a on a.rule_id = r.id").
		Where(squirrel.Eq{
			"r.scenario_id": scenarioId,
		}).
		GroupBy("r.id").
		OrderBy("max(r.priority)")

	return SqlToListOfModels(ctx, exec, sql, dbmodels.AdaptWorkflowRuleWithConditions)
}

func (repo *MarbleDbRepository) GetWorkflowRule(ctx context.Context, exec Executor, id string) (models.WorkflowRule, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return models.WorkflowRule{}, err
	}

	sql := NewQueryBuilder().
		Select(dbmodels.WorkflowRuleColumns...).
		From(dbmodels.TABLE_WORKFLOW_RULES).
		Where("id = ?", id)

	return SqlToModel(ctx, exec, sql, dbmodels.AdaptWorkflowRule)
}

func (repo *MarbleDbRepository) GetWorkflowCondition(ctx context.Context, exec Executor, id string) (models.WorkflowCondition, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return models.WorkflowCondition{}, err
	}

	sql := NewQueryBuilder().
		Select(dbmodels.WorkflowConditionColumns...).
		From(dbmodels.TABLE_WORKFLOW_CONDITIONS).
		Where("id = ?", id)

	return SqlToModel(ctx, exec, sql, dbmodels.AdaptWorkflowCondition)
}

func (repo *MarbleDbRepository) GetWorkflowAction(ctx context.Context, exec Executor, id string) (models.WorkflowAction, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return models.WorkflowAction{}, err
	}

	sql := NewQueryBuilder().
		Select(dbmodels.WorkflowConditionColumns...).
		From(dbmodels.TABLE_WORKFLOW_ACTIONS).
		Where("id = ?", id)

	return SqlToModel(ctx, exec, sql, dbmodels.AdaptWorkflowAction)
}

func (repo *MarbleDbRepository) InsertWorkflowRule(ctx context.Context, exec Executor, rule models.WorkflowRule) (models.WorkflowRule, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return models.WorkflowRule{}, err
	}

	sql := NewQueryBuilder().
		Insert(dbmodels.TABLE_WORKFLOW_RULES).
		Columns("scenario_id", "name", "priority").
		Values(
			rule.ScenarioId,
			rule.Name,
			squirrel.Expr("(select coalesce(max(priority), 0) + 1 from "+dbmodels.TABLE_WORKFLOW_RULES+" where scenario_id = ?)", rule.ScenarioId),
		).
		Suffix("returning *")

	return SqlToModel(ctx, exec, sql, dbmodels.AdaptWorkflowRule)
}

func (repo *MarbleDbRepository) UpdateWorkflowRule(ctx context.Context, exec Executor, rule models.WorkflowRule) (models.WorkflowRule, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return models.WorkflowRule{}, err
	}

	sql := NewQueryBuilder().
		Update(dbmodels.TABLE_WORKFLOW_RULES).
		SetMap(map[string]any{
			"name": rule.Name,
		}).
		Where("scenario_id = ?", rule.ScenarioId).
		Where("id = ?", rule.Id).
		Suffix("returning *")

	return SqlToModel(ctx, exec, sql, dbmodels.AdaptWorkflowRule)
}

func (repo *MarbleDbRepository) InsertWorkflowCondition(ctx context.Context, exec Executor, cond models.WorkflowCondition) (models.WorkflowCondition, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return models.WorkflowCondition{}, err
	}

	sql := NewQueryBuilder().
		Insert(dbmodels.TABLE_WORKFLOW_CONDITIONS).
		Columns("rule_id", "function", "params").
		Values(
			cond.RuleId,
			cond.Function,
			cond.Params,
		).
		Suffix("returning *")

	return SqlToModel(ctx, exec, sql, dbmodels.AdaptWorkflowCondition)
}

func (repo *MarbleDbRepository) InsertWorkflowAction(ctx context.Context, exec Executor, action models.WorkflowAction) (models.WorkflowAction, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return models.WorkflowAction{}, err
	}

	sql := NewQueryBuilder().
		Insert(dbmodels.TABLE_WORKFLOW_ACTIONS).
		Columns("rule_id", "action", "params").
		Values(
			action.RuleId,
			action.Action,
			action.Params,
		).
		Suffix("returning *")

	return SqlToModel(ctx, exec, sql, dbmodels.AdaptWorkflowAction)
}

func (repo *MarbleDbRepository) ReorderWorkflowRules(ctx context.Context, exec Executor, scenarioId string, ids []uuid.UUID) error {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return err
	}

	sql := NewQueryBuilder().
		Update(dbmodels.TABLE_WORKFLOW_RULES).
		Set("priority", squirrel.Expr("coalesce(array_position(?, id), 99)", ids)).
		Where("scenario_id = ?", scenarioId)

	return ExecBuilder(ctx, exec, sql)
}
