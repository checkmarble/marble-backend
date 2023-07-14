package repositories

import (
	"marble/marble-backend/models"
	"marble/marble-backend/repositories/dbmodels"
)

type ScenarioWriteRepository interface {
	CreateScenario(tx Transaction, scenario models.CreateScenarioInput, newScenarioId string) error
	UpdateScenario(tx Transaction, scenario models.UpdateScenarioInput) error
	UpdateScenarioLiveItereationId(tx Transaction, scenarioId string, scenarioIterationId *string) error
}

type ScenarioWriteRepositoryPostgresql struct {
	transactionFactory TransactionFactory
}

func NewScenarioWriteRepositoryPostgresql(transactionFactory TransactionFactory) ScenarioWriteRepository {
	return &ScenarioWriteRepositoryPostgresql{
		transactionFactory: transactionFactory,
	}
}

func (repo *ScenarioWriteRepositoryPostgresql) CreateScenario(tx Transaction, scenario models.CreateScenarioInput, newScenarioId string) error {
	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(tx)

	_, err := pgTx.ExecBuilder(
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
				scenario.OrganizationID,
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

func (repo *ScenarioWriteRepositoryPostgresql) UpdateScenario(tx Transaction, scenario models.UpdateScenarioInput) error {
	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(tx)

	sql := NewQueryBuilder().
		Update(dbmodels.TABLE_SCENARIOS).
		Where("id = ?", scenario.ID)

	if scenario.Name != nil {
		sql = sql.Set("name", scenario.Name)
	}
	if scenario.Description != nil {
		sql = sql.Set("description", scenario.Description)
	}

	if _, err := pgTx.ExecBuilder(sql); err != nil {
		return err
	}

	return nil
}

func (repo *ScenarioWriteRepositoryPostgresql) UpdateScenarioLiveItereationId(tx Transaction, scenarioId string, scenarioIterationId *string) error {
	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(tx)

	sql := NewQueryBuilder().
		Update(dbmodels.TABLE_SCENARIOS).
		Where("id = ?", scenarioId).
		Set("live_scenario_iteration_id", scenarioIterationId)

	if _, err := pgTx.ExecBuilder(sql); err != nil {
		return err
	}
	return nil
}
