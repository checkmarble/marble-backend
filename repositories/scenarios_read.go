package repositories

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories/dbmodels"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/Masterminds/squirrel"
)

type ScenarioUsecaseRepository interface {
	GetScenarioById(ctx context.Context, exec Executor, scenarioId string) (models.Scenario, error)
	ListScenariosOfOrganization(ctx context.Context, exec Executor, organizationId uuid.UUID) ([]models.Scenario, error)
	CreateScenario(
		ctx context.Context,
		exec Executor,
		organizationId uuid.UUID,
		scenario models.CreateScenarioInput,
		newScenarioId string,
	) error
	UpdateScenario(
		ctx context.Context,
		exec Executor,
		scenario models.UpdateScenarioInput,
	) error
	ListScenarioLatestRuleVersions(ctx context.Context, exec Executor, scenarioId string) (
		[]models.ScenarioRuleLatestVersion, error)
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

func (repo *MarbleDbRepository) ListScenariosOfOrganization(ctx context.Context, exec Executor, organizationId uuid.UUID) ([]models.Scenario, error) {
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

	if filters.OrganizationId != nil {
		query = query.Where(squirrel.Eq{"org_id": *filters.OrganizationId})
	}

	return SqlToListOfModels(
		ctx,
		exec,
		query,
		dbmodels.AdaptScenario,
	)
}

// ListLiveIterationsAndNeighbors returns a list of scenario iterations,
// whatever the scenarios is, that may considered live-adjacent. It returns the
// live iterations and all iterations that were live within a time period before
// the current time.
//
// The final query looks like this (useful for debugging):
/*
	with
	  live as (
	    select si.id as iteration_id
	    from scenario_iterations si
	    inner join scenarios s on s.live_scenario_iteration_id = si.id
	    where si.org_id = '<org_id>'
	  ),
	  test_runs as (
	    select str.scenario_iteration_id as iteration_id
	    from scenario_test_run str
	    inner join scenario_iterations sti on sti.id = str.live_scenario_iteration_id
	    where
	      sti.org_id = '<org_id>' and
	      status = 'up'
	  ),
	  neighbors as (
	    select sp.scenario_iteration_id as iteration_id
	    from scenario_publications sp
	    where
	      org_id = '<org_id>' and
	      publication_action in ('publish', 'prepare') and
	      created_at > now() - interval '72 hour'
	  )
	select
	  si.*
	  array_agg(row(sir.*)) filter (where sir.id is not null) as rules
	from scenario_iterations si
	left join scenario_iteration_rules AS sir on sir.scenario_iteration_id = si.id
	where si.id in (
	  select iteration_id from live
	  union
	  select iteration_id from neighbors
	  union
	  select iteration_id from test_runs
	)
	group by si.id;
*/
func (repo *MarbleDbRepository) ListLiveIterationsAndNeighbors(ctx context.Context,
	exec Executor, orgId uuid.UUID,
) ([]models.ScenarioIteration, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, err
	}

	ctes := WithCtes("live", func(b squirrel.StatementBuilderType) squirrel.SelectBuilder {
		return b.
			Select("si.id as id").
			From(dbmodels.TABLE_SCENARIO_ITERATIONS+" si").
			InnerJoin("scenarios s on s.live_scenario_iteration_id = si.id").
			Where("si.org_id = ?", orgId)
	}).
		With("test_run", func(b squirrel.StatementBuilderType) squirrel.SelectBuilder {
			return b.
				Select("str.scenario_iteration_id as id").
				From(dbmodels.TABLE_SCENARIO_ITERATIONS+" si").
				InnerJoin(dbmodels.TABLE_SCENARIO_TESTRUN+
					" str on str.scenario_iteration_id = si.id").
				Where("si.org_id = ? and str.status = 'up'", orgId)
		}).
		With("neighbors", func(b squirrel.StatementBuilderType) squirrel.SelectBuilder {
			return b.
				Select("sp.scenario_iteration_id as id").
				From(dbmodels.TABLE_SCENARIOS_PUBLICATIONS + " sp").
				Where(squirrel.And{
					squirrel.Eq{
						"org_id": orgId,
						"publication_action": []string{
							models.Prepare.String(),
							models.Publish.String(), models.Unpublish.String(),
						},
					},
					squirrel.Gt{
						"created_at": time.Now().Add(-3 * 24 * time.Hour),
					},
				})
		})

	sql := NewQueryBuilder().
		Select(columnsNames("si", dbmodels.SelectScenarioIterationColumn)...).
		PrefixExpr(ctes).
		Column(
			fmt.Sprintf(
				"array_agg(row(%s)) filter (where sir.id is not null) as rules",
				strings.Join(columnsNames("sir", dbmodels.SelectRulesColumn), ","),
			),
		).
		From(dbmodels.TABLE_SCENARIO_ITERATIONS + " si").
		LeftJoin(dbmodels.TABLE_RULES + " AS sir ON sir.scenario_iteration_id = si.id").
		Where("si.id in (select id from live union select id from test_run union select id from neighbors)").
		GroupBy("si.id")

	return SqlToListOfModels(ctx, exec, sql, dbmodels.AdaptScenarioIterationWithRules)
}

func (repo *MarbleDbRepository) ListScenarioLatestRuleVersions(ctx context.Context, exec Executor,
	scenarioId string,
) ([]models.ScenarioRuleLatestVersion, error) {
	sql := `
		select type, stable_rule_id, name, version
		from (
			select
				rank() over (partition by sir.stable_rule_id order by si.version desc) as rank,
				'rule' as type,
				sir.stable_rule_id,
				sir.name,
				si.version
			from scenario_iteration_rules sir
			inner join scenario_iterations si on si.id = sir.scenario_iteration_id
			where si.scenario_id = $1
			and si.version is not null
			union all
			select
				rank() over (partition by scc.stable_id order by si.version desc) as rank,
				'screening' as type,
				scc.stable_id,
				scc.name,
				si.version
			from screening_configs scc
			inner join scenario_iterations si on si.id = scc.scenario_iteration_id
			where si.scenario_id = $2
			and si.version is not null
		) rules
		where rules.rank = 1
		order by rules.version desc, rules.name asc
	`

	rows, err := exec.Query(ctx, sql, scenarioId, scenarioId)
	if err != nil {
		return nil, err
	}

	rules := make([]models.ScenarioRuleLatestVersion, 0)

	for rows.Next() {
		rule, err := pgx.RowToStructByName[dbmodels.DBScenarioRuleLatestVersion](rows)
		if err != nil {
			return nil, err
		}

		rules = append(rules, models.ScenarioRuleLatestVersion{
			Type:          rule.Type,
			StableId:      rule.StableRuleId,
			Name:          rule.Name,
			LatestVersion: rule.Version,
		})
	}

	return rules, nil
}
