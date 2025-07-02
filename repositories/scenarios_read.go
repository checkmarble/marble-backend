package repositories

import (
	"context"
	"fmt"
	"strings"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories/dbmodels"

	"github.com/Masterminds/squirrel"
)

type ScenarioUsecaseRepository interface {
	GetScenarioById(ctx context.Context, exec Executor, scenarioId string) (models.Scenario, error)
	ListScenariosOfOrganization(ctx context.Context, exec Executor, organizationId string) ([]models.Scenario, error)
	CreateScenario(
		ctx context.Context,
		exec Executor,
		organizationId string,
		scenario models.CreateScenarioInput,
		newScenarioId string,
	) error
	UpdateScenario(
		ctx context.Context,
		exec Executor,
		scenario models.UpdateScenarioInput,
	) error
}

func selectScenarios() squirrel.SelectBuilder {
	return NewQueryBuilder().
		Select(dbmodels.SelectScenarioColumn...).
		From(dbmodels.TABLE_SCENARIOS)
}

func (repo *MarbleDbRepository) GetScenarioById(ctx context.Context, exec Executor, scenarioId string) (models.Scenario, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return models.Scenario{}, err
	}

	return SqlToModel(
		ctx,
		exec,
		selectScenarios().Where(squirrel.Eq{"id": scenarioId}),
		dbmodels.AdaptScenario,
	)
}

func (repo *MarbleDbRepository) ListScenariosOfOrganization(ctx context.Context, exec Executor, organizationId string) ([]models.Scenario, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, err
	}

	return SqlToListOfModels(
		ctx,
		exec,
		selectScenarios().Where(squirrel.Eq{"org_id": organizationId}).OrderBy("created_at DESC"),
		dbmodels.AdaptScenario,
	)
}

func (repo *MarbleDbRepository) ListAllScenarios(ctx context.Context, exec Executor,
	filters models.ListAllScenariosFilters,
) ([]models.Scenario, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, err
	}

	query := selectScenarios().OrderBy("id")

	if filters.Live {
		query = query.Where(squirrel.NotEq{"live_scenario_iteration_id": nil})
	}

	return SqlToListOfModels(
		ctx,
		exec,
		query,
		dbmodels.AdaptScenario,
	)
}

// ListLiveIterationsAndNeighbors returns a list of scenario iterations, whatever the scenarios is,
// that may considered live-adjacent.
// It obviously returns the actual live iterations, but also one previous version and all next versions.
// For example, if a scenario has a live iteration of 10, but has iterations from 1 to 13, this will returns
// iterations 9 to 13.
//
// The final query looks like this (useful for debugging):
/*
	with live as (
		select scenario_id, si.id as live_id str.scenario_iteration_id as test_run_id, version, si.updated_at
		from scenario_iterations si
		inner join scenarios s on s.live_scenario_iteration_id = si.id
		left join scenario_test_run str ON str.live_scenario_iteration_id = s.live_scenario_iteration_id and str.status = 'up'
		where si.org_id = '<org_id>')
	select
		si.id,
		si.org_id,
		si.scenario_id,
		si.version,
		si.created_at,
		si.updated_at,
		si.score_review_threshold,
		si.score_block_and_review_threshold,
		si.score_reject_threshold,
		si.trigger_condition_ast_expression,
		si.deleted_at,
		si.schedule,
		l.test_run_id,
		array_agg(row(sir.id,sir.org_id,sir.scenario_iteration_id,sir.display_order,sir.name,sir.description,sir.score_modifier,sir.formula_ast_expression,sir.created_at,sir.deleted_at,sir.rule_group,sir.snooze_group_id,sir.stable_rule_id)) filter (where sir.id is not null) as rules
	from scenario_iterations si
	inner join live l on l.scenario_id = si.scenario_id or l.test_run_id = si.id
	left join scenario_iteration_rules AS sir on sir.scenario_iteration_id = si.id
	where si.version is not null and (si.id = l.live_id or si.version >= l.version or si.updated_at > l.updated_at or si.id = l.test_run_id)
	group by si.id, l.test_run_id;
*/
func (repo *MarbleDbRepository) ListLiveIterationsAndNeighbors(ctx context.Context,
	exec Executor, orgId string,
) ([]models.ScenarioIteration, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, err
	}

	liveCte := NewQueryBuilder().
		Select("scenario_id", "si.id as live_id", "str.scenario_iteration_id as test_run_id", "version").
		From(dbmodels.TABLE_SCENARIO_ITERATIONS + " si").
		InnerJoin("scenarios s on s.live_scenario_iteration_id = si.id").
		LeftJoin("scenario_test_run str on str.live_scenario_iteration_id = s.live_scenario_iteration_id and str.status = 'up'").
		Where(squirrel.Eq{"si.org_id": orgId}).Prefix("with live as(").Suffix(")")

	sql := NewQueryBuilder().
		Select(columnsNames("si", dbmodels.SelectScenarioIterationColumn)...).
		PrefixExpr(liveCte).
		Column(
			fmt.Sprintf(
				"array_agg(row(%s)) filter (where sir.id is not null) as rules",
				strings.Join(columnsNames("sir", dbmodels.SelectRulesColumn), ","),
			),
		).
		From(dbmodels.TABLE_SCENARIO_ITERATIONS + " si").
		InnerJoin("live l on l.scenario_id = si.scenario_id").
		LeftJoin(dbmodels.TABLE_RULES + " AS sir ON sir.scenario_iteration_id = si.id").
		Where(squirrel.And{
			squirrel.NotEq{"si.version": nil},
			squirrel.Expr("si.id = l.live_id or si.version >= l.version - 1 or si.id = l.test_run_id"),
		}).
		GroupBy("si.id")

	return SqlToListOfModels(ctx, exec, sql, dbmodels.AdaptScenarioIterationWithRules)
}
