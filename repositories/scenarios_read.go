package repositories

import (
	"context"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories/dbmodels"

	"github.com/Masterminds/squirrel"
)

type ScenarioUsecaseRepository interface {
	GetScenarioById(ctx context.Context, exec Executor, scenarioId string) (models.Scenario, error)
	GetScenarioByLiveScenarioIterationId(ctx context.Context,
		exec Executor, scenarioIterationId string,
	) (models.Scenario, error)
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

func (repo *MarbleDbRepository) GetScenarioByLiveScenarioIterationId(ctx context.Context,
	exec Executor, scenarioIterationId string,
) (models.Scenario, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return models.Scenario{}, err
	}

	return SqlToModel(
		ctx,
		exec,
		selectScenarios().Where(squirrel.Eq{"live_scenario_iteration_id": scenarioIterationId}),
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
