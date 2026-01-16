package repositories

import (
	"context"

	"github.com/Masterminds/squirrel"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories/dbmodels"
	"github.com/google/uuid"
)

func (repo *MarbleDbRepository) ListAllOrgWorkflows(ctx context.Context, exec Executor, orgId uuid.UUID) ([]models.Workflow, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, err
	}

	sql := NewQueryBuilder().
		Select(columnsNames("r", dbmodels.WorkflowRuleColumns)...).
		Column("array_agg(distinct row(c.*)) filter (where c.id is not null) as conditions").
		Column("array_agg(distinct row(a.*)) filter (where a.id is not null) as actions").
		From(dbmodels.TABLE_WORKFLOW_RULES+" r").
		LeftJoin(dbmodels.TABLE_WORKFLOW_CONDITIONS+" c on c.rule_id = r.id").
		LeftJoin(dbmodels.TABLE_WORKFLOW_ACTIONS+" a on a.rule_id = r.id").
		LeftJoin(dbmodels.TABLE_SCENARIOS+" s on s.id = r.scenario_id").
		Where("s.org_id = ?", orgId).
		GroupBy("r.id")

	return SqlToListOfModels(ctx, exec, sql, dbmodels.AdaptWorkflowRuleWithConditions)
}

func (repo *MarbleDbRepository) ListWorkflowsForScenario(ctx context.Context, exec Executor, scenarioId uuid.UUID) ([]models.Workflow, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, err
	}

	sql := NewQueryBuilder().
		Select(columnsNames("r", dbmodels.WorkflowRuleColumns)...).
		Column("array_agg(distinct row(c.*)) filter (where c.id is not null) as conditions").
		Column("array_agg(distinct row(a.*)) filter (where a.id is not null) as actions").
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

func (repo *MarbleDbRepository) GetWorkflowRule(ctx context.Context, exec Executor, id uuid.UUID) (models.WorkflowRule, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return models.WorkflowRule{}, err
	}

	sql := NewQueryBuilder().
		Select(dbmodels.WorkflowRuleColumns...).
		From(dbmodels.TABLE_WORKFLOW_RULES).
		Where("id = ?", id)

	return SqlToModel(ctx, exec, sql, dbmodels.AdaptWorkflowRule)
}

func (repo *MarbleDbRepository) GetWorkflowRuleDetails(ctx context.Context, exec Executor, id uuid.UUID) (models.Workflow, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return models.Workflow{}, err
	}

	sql := NewQueryBuilder().
		Select(columnsNames("r", dbmodels.WorkflowRuleColumns)...).
		Column("array_agg(distinct row(c.*)) filter (where c.id is not null) as conditions").
		Column("array_agg(distinct row(a.*)) filter (where a.id is not null) as actions").
		From(dbmodels.TABLE_WORKFLOW_RULES + " r").
		LeftJoin(dbmodels.TABLE_WORKFLOW_CONDITIONS + " c on c.rule_id = r.id").
		LeftJoin(dbmodels.TABLE_WORKFLOW_ACTIONS + " a on a.rule_id = r.id").
		Where(squirrel.Eq{"r.id": id}).
		GroupBy("r.id")

	return SqlToModel(ctx, exec, sql, dbmodels.AdaptWorkflowRuleWithConditions)
}

func (repo *MarbleDbRepository) GetWorkflowCondition(ctx context.Context, exec Executor, id uuid.UUID) (models.WorkflowCondition, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return models.WorkflowCondition{}, err
	}

	sql := NewQueryBuilder().
		Select(dbmodels.WorkflowConditionColumns...).
		From(dbmodels.TABLE_WORKFLOW_CONDITIONS).
		Where("id = ?", id)

	return SqlToModel(ctx, exec, sql, dbmodels.AdaptWorkflowCondition)
}

func (repo *MarbleDbRepository) GetWorkflowAction(ctx context.Context, exec Executor, id uuid.UUID) (models.WorkflowAction, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return models.WorkflowAction{}, err
	}

	sql := NewQueryBuilder().
		Select(dbmodels.WorkflowActionColumns...).
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
		Columns("scenario_id", "name", "priority", "fallthrough").
		Values(
			rule.ScenarioId,
			rule.Name,
			squirrel.Expr("(select coalesce(max(priority), 0) + 1 from "+dbmodels.TABLE_WORKFLOW_RULES+" where scenario_id = ?)", rule.ScenarioId),
			rule.Fallthrough,
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
			"name":        rule.Name,
			"fallthrough": rule.Fallthrough,
		}).
		Where("id = ?", rule.Id).
		Suffix("returning *")

	return SqlToModel(ctx, exec, sql, dbmodels.AdaptWorkflowRule)
}

func (repo *MarbleDbRepository) DeleteWorkflowRule(ctx context.Context, exec Executor, ruleId uuid.UUID) error {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return err
	}

	sql := NewQueryBuilder().
		Delete(dbmodels.TABLE_WORKFLOW_RULES).
		Where("id = ?", ruleId)

	return ExecBuilder(ctx, exec, sql)
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

func (repo *MarbleDbRepository) UpdateWorkflowCondition(ctx context.Context, exec Executor, rule models.WorkflowCondition) (models.WorkflowCondition, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return models.WorkflowCondition{}, err
	}

	sql := NewQueryBuilder().
		Update(dbmodels.TABLE_WORKFLOW_CONDITIONS).
		SetMap(map[string]any{
			"function": rule.Function,
			"params":   rule.Params,
		}).
		Where("rule_id = ?", rule.RuleId).
		Where("id = ?", rule.Id).
		Suffix("returning *")

	return SqlToModel(ctx, exec, sql, dbmodels.AdaptWorkflowCondition)
}

func (repo *MarbleDbRepository) DeleteWorkflowCondition(ctx context.Context, exec Executor, ruleId, conditionId uuid.UUID) error {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return err
	}

	sql := NewQueryBuilder().
		Delete(dbmodels.TABLE_WORKFLOW_CONDITIONS).
		Where("rule_id = ?", ruleId).
		Where("id = ?", conditionId)

	return ExecBuilder(ctx, exec, sql)
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

func (repo *MarbleDbRepository) UpdateWorkflowAction(ctx context.Context, exec Executor, rule models.WorkflowAction) (models.WorkflowAction, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return models.WorkflowAction{}, err
	}

	sql := NewQueryBuilder().
		Update(dbmodels.TABLE_WORKFLOW_ACTIONS).
		SetMap(map[string]any{
			"action": rule.Action,
			"params": rule.Params,
		}).
		Where("rule_id = ?", rule.RuleId).
		Where("id = ?", rule.Id).
		Suffix("returning *")

	return SqlToModel(ctx, exec, sql, dbmodels.AdaptWorkflowAction)
}

func (repo *MarbleDbRepository) DeleteWorkflowAction(ctx context.Context, exec Executor, ruleId, actionId uuid.UUID) error {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return err
	}

	sql := NewQueryBuilder().
		Delete(dbmodels.TABLE_WORKFLOW_ACTIONS).
		Where("rule_id = ?", ruleId).
		Where("id = ?", actionId)

	return ExecBuilder(ctx, exec, sql)
}

func (repo *MarbleDbRepository) ReorderWorkflowRules(ctx context.Context, exec Executor, scenarioId uuid.UUID, ids []uuid.UUID) error {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return err
	}

	sql := NewQueryBuilder().
		Update(dbmodels.TABLE_WORKFLOW_RULES).
		Set("priority", squirrel.Expr("coalesce(array_position(?, id), 99)", ids)).
		Where("scenario_id = ?", scenarioId)

	return ExecBuilder(ctx, exec, sql)
}
