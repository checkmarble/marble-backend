package repositories

import (
	"context"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories/dbmodels"

	"github.com/Masterminds/squirrel"
)

type ScenarioPublicationRepository interface {
	ListScenarioPublicationsOfOrganization(ctx context.Context, exec Executor, organizationId string,
		filters models.ListScenarioPublicationsFilters) ([]models.ScenarioPublication, error)
	CreateScenarioPublication(ctx context.Context, exec Executor,
		input models.CreateScenarioPublicationInput, newScenarioPublicationId string) error
	GetScenarioPublicationById(ctx context.Context, exec Executor, scenarioPublicationID string) (models.ScenarioPublication, error)
}

type ScenarioPublicationRepositoryPostgresql struct{}

func selectScenarioPublications() squirrel.SelectBuilder {
	return NewQueryBuilder().
		Select(dbmodels.SelectScenarioPublicationColumns...).
		From(dbmodels.TABLE_SCENARIOS_PUBLICATIONS)
}

func (repo *ScenarioPublicationRepositoryPostgresql) GetScenarioPublicationById(ctx context.Context,
	exec Executor, scenarioPublicationID string,
) (models.ScenarioPublication, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return models.ScenarioPublication{}, err
	}

	return SqlToModel(
		ctx,
		exec,
		selectScenarioPublications().Where(squirrel.Eq{"id": scenarioPublicationID}),
		dbmodels.AdaptScenarioPublication,
	)
}

func (repo *ScenarioPublicationRepositoryPostgresql) ListScenarioPublicationsOfOrganization(
	ctx context.Context, exec Executor, organizationId string, filters models.ListScenarioPublicationsFilters,
) ([]models.ScenarioPublication, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, err
	}

	query := selectScenarioPublications().
		Where(squirrel.Eq{"org_id": organizationId}).
		OrderBy("rank ASC")

	if filters.ScenarioId != nil {
		query = query.Where(squirrel.Eq{"scenario_id": *filters.ScenarioId})
	}
	if filters.ScenarioIterationId != nil {
		query = query.Where(squirrel.Eq{"scenario_iteration_id": *filters.ScenarioIterationId})
	}

	return SqlToListOfModels(
		ctx,
		exec,
		query,
		dbmodels.AdaptScenarioPublication,
	)
}

func (repo *ScenarioPublicationRepositoryPostgresql) CreateScenarioPublication(ctx context.Context,
	exec Executor, input models.CreateScenarioPublicationInput, newScenarioPublicationId string,
) error {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return err
	}

	err := ExecBuilder(
		ctx,
		exec,
		NewQueryBuilder().Insert(dbmodels.TABLE_SCENARIOS_PUBLICATIONS).
			Columns(
				"id",
				"org_id",
				"scenario_id",
				"scenario_iteration_id",
				"publication_action",
			).
			Values(
				newScenarioPublicationId,
				input.OrganizationId,
				input.ScenarioId,
				input.ScenarioIterationId,
				input.PublicationAction.String(),
			),
	)
	return err
}
