package repositories

import (
	"context"

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
	if scenario.DecisionToCaseInboxId.Valid {
		if scenario.DecisionToCaseInboxId.String == "" {
			sql = sql.Set("decision_to_case_inbox_id", nil)
		} else {
			sql = sql.Set("decision_to_case_inbox_id", scenario.DecisionToCaseInboxId)
		}
		countApply++
	}
	if scenario.DecisionToCaseOutcomes != nil {
		sql = sql.Set("decision_to_case_outcomes", scenario.DecisionToCaseOutcomes)
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
