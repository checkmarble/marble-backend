package repositories

import (
	"context"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories/dbmodels"

	"github.com/Masterminds/squirrel"
)

type ScenarioPublicationRepository interface {
	ListScenarioPublicationsOfOrganization(ctx context.Context, tx Transaction_deprec, organizationId string, filters models.ListScenarioPublicationsFilters) ([]models.ScenarioPublication, error)
	CreateScenarioPublication(ctx context.Context, tx Transaction_deprec, input models.CreateScenarioPublicationInput, newScenarioPublicationId string) error
	GetScenarioPublicationById(ctx context.Context, tx Transaction_deprec, scenarioPublicationID string) (models.ScenarioPublication, error)
}

type ScenarioPublicationRepositoryPostgresql struct {
	transactionFactory TransactionFactoryPosgresql_deprec
}

func NewScenarioPublicationRepositoryPostgresql(transactionFactory TransactionFactoryPosgresql_deprec) ScenarioPublicationRepository {
	return &ScenarioPublicationRepositoryPostgresql{
		transactionFactory: transactionFactory,
	}
}

func selectScenarioPublications() squirrel.SelectBuilder {
	return NewQueryBuilder().
		Select(dbmodels.SelectScenarioPublicationColumns...).
		From(dbmodels.TABLE_SCENARIOS_PUBLICATIONS)
}

func (repo *ScenarioPublicationRepositoryPostgresql) GetScenarioPublicationById(ctx context.Context, tx Transaction_deprec, scenarioPublicationID string) (models.ScenarioPublication, error) {
	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(ctx, tx)

	return SqlToModel(
		ctx,
		pgTx,
		selectScenarioPublications().Where(squirrel.Eq{"id": scenarioPublicationID}),
		dbmodels.AdaptScenarioPublication,
	)
}

func (repo *ScenarioPublicationRepositoryPostgresql) ListScenarioPublicationsOfOrganization(ctx context.Context, tx Transaction_deprec, organizationId string, filters models.ListScenarioPublicationsFilters) ([]models.ScenarioPublication, error) {
	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(ctx, tx)

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
		pgTx,
		query,
		dbmodels.AdaptScenarioPublication,
	)
}

func (repo *ScenarioPublicationRepositoryPostgresql) CreateScenarioPublication(ctx context.Context, tx Transaction_deprec, input models.CreateScenarioPublicationInput, newScenarioPublicationId string) error {
	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(ctx, tx)

	_, err := pgTx.ExecBuilder(
		ctx,
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
