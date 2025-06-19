package repositories

import (
	"context"
	"fmt"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories/dbmodels"
)

func (repo *MarbleDbRepository) CreateScenario(ctx context.Context, exec Executor,
	organizationId string, scenario models.CreateScenarioInput, newScenarioId string,
) error {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return err
	}

	err := ExecBuilder(
		ctx,
		exec,
		NewQueryBuilder().Insert(dbmodels.TABLE_SCENARIOS).
			Columns(
				"id",
				"org_id",
				"name",
				"description",
				"trigger_object_type",
			).
			Values(
				newScenarioId,
				organizationId,
				scenario.Name,
				scenario.Description,
				scenario.TriggerObjectType,
			),
	)
	if err != nil {
		return err
	}

	return nil
}

func (repo *MarbleDbRepository) UpdateScenario(ctx context.Context, exec Executor, scenario models.UpdateScenarioInput) error {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return err
	}

	sql := NewQueryBuilder().
		Update(dbmodels.TABLE_SCENARIOS).
		Where("id = ?", scenario.Id)

	countApply := 0
	if scenario.DecisionToCaseInboxId.Set {
		sql = sql.Set("decision_to_case_inbox_id", scenario.DecisionToCaseInboxId.Value())
		countApply++
	}
	if scenario.DecisionToCaseOutcomes != nil {
		sql = sql.Set("decision_to_case_outcomes", scenario.DecisionToCaseOutcomes)
		countApply++
	}
	if scenario.DecisionToCaseWorkflowType != nil {
		sql = sql.Set("decision_to_case_workflow_type", scenario.DecisionToCaseWorkflowType)
		countApply++
	}
	if scenario.DecisionToCaseNameTemplate != nil {
		serializedAst, err := dbmodels.SerializeFormulaAstExpression(scenario.DecisionToCaseNameTemplate)
		if err != nil {
			return fmt.Errorf(
				"unable to marshal ast expression: %w", err)
		}
		sql = sql.Set("decision_to_case_name_template", serializedAst)
		countApply++
	}
	if scenario.Description != nil {
		sql = sql.Set("description", scenario.Description)
		countApply++
	}
	if scenario.Name != nil {
		sql = sql.Set("name", scenario.Name)
		countApply++
	}

	if countApply == 0 {
		return nil
	}

	if err := ExecBuilder(ctx, exec, sql); err != nil {
		return err
	}

	return nil
}

func (repo *MarbleDbRepository) UpdateScenarioLiveIterationId(ctx context.Context, exec Executor, scenarioId string, scenarioIterationId *string) error {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return err
	}

	sql := NewQueryBuilder().
		Update(dbmodels.TABLE_SCENARIOS).
		Where("id = ?", scenarioId).
		Set("live_scenario_iteration_id", scenarioIterationId)

	if err := ExecBuilder(ctx, exec, sql); err != nil {
		return err
	}
	return nil
}
