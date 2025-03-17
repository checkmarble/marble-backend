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
func (repo *MarbleDbRepository) ListLiveIterationsAndNeighbors(ctx context.Context,
	exec Executor, orgId string,
) ([]models.ScenarioIteration, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, err
	}

	liveCte := NewQueryBuilder().
		Select("scenario_id", "version").
		From(dbmodels.TABLE_SCENARIO_ITERATIONS + " si").
		InnerJoin("scenarios s on s.live_scenario_iteration_id = si.id").
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
			squirrel.Expr("si.version >= l.version - 1"),
		}).
		GroupBy("si.id")

	return SqlToListOfModels(ctx, exec, sql, dbmodels.AdaptScenarioIterationWithRules)
}
