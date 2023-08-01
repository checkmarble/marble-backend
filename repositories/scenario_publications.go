package repositories

import (
	"marble/marble-backend/models"
	"marble/marble-backend/repositories/dbmodels"

	"github.com/Masterminds/squirrel"
)

type ScenarioPublicationRepository interface {
	ListScenarioPublicationsOfOrganization(tx Transaction, organizationId string, filters models.ListScenarioPublicationsFilters) ([]models.ScenarioPublication, error)
	CreateScenarioPublication(tx Transaction, input models.CreateScenarioPublicationInput, newScenarioPublicationId string) error
	GetScenarioPublicationById(tx Transaction, scenarioPublicationID string) (models.ScenarioPublication, error)
	// FIXME Just temporarily placed here, will be moved to scenario iteration write repo
	UpdateScenarioIterationVersion(tx Transaction, scenarioIterationId string, newVersion int) error
}

type ScenarioPublicationRepositoryPostgresql struct {
	transactionFactory TransactionFactory
}

func NewScenarioPublicationRepositoryPostgresql(transactionFactory TransactionFactory) ScenarioPublicationRepository {
	return &ScenarioPublicationRepositoryPostgresql{
		transactionFactory: transactionFactory,
	}
}

func selectScenarioPublications() squirrel.SelectBuilder {
	return NewQueryBuilder().
		Select(dbmodels.SelectScenarioPublicationColumns...).
		From(dbmodels.TABLE_SCENARIOS_PUBLICATIONS)
}

func (repo *ScenarioPublicationRepositoryPostgresql) GetScenarioPublicationById(tx Transaction, scenarioPublicationID string) (models.ScenarioPublication, error) {
	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(tx)

	return SqlToModel(
		pgTx,
		selectScenarioPublications().Where(squirrel.Eq{"id": scenarioPublicationID}),
		dbmodels.AdaptScenarioPublication,
	)
}

func (repo *ScenarioPublicationRepositoryPostgresql) ListScenarioPublicationsOfOrganization(tx Transaction, organizationId string, filters models.ListScenarioPublicationsFilters) ([]models.ScenarioPublication, error) {
	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(tx)

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
		pgTx,
		query,
		dbmodels.AdaptScenarioPublication,
	)
}

func (repo *ScenarioPublicationRepositoryPostgresql) CreateScenarioPublication(tx Transaction, input models.CreateScenarioPublicationInput, newScenarioPublicationId string) error {
	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(tx)

	_, err := pgTx.ExecBuilder(
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

// FIXME Just temporarily placed here, will be moved to scenario iteration write repo
func (repo *ScenarioPublicationRepositoryPostgresql) UpdateScenarioIterationVersion(tx Transaction, scenarioIterationId string, newVersion int) error {
	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(tx)

	_, err := pgTx.ExecBuilder(
		NewQueryBuilder().Update(dbmodels.TABLE_SCENARIO_ITERATIONS).
			Set("version", newVersion).
			Where(squirrel.Eq{"id": scenarioIterationId}),
	)
	return err
}
