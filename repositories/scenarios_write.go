package repositories

import (
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories/dbmodels"
)

type ScenarioWriteRepository interface {
	CreateScenario(tx Transaction, organizationId string, scenario models.CreateScenarioInput, newScenarioId string) error
	UpdateScenario(tx Transaction, scenario models.UpdateScenarioInput) error
	UpdateScenarioLiveIterationId(tx Transaction, scenarioId string, scenarioIterationId *string) error
}

type ScenarioWriteRepositoryPostgresql struct {
	transactionFactory TransactionFactoryPosgresql
}

func NewScenarioWriteRepositoryPostgresql(transactionFactory TransactionFactoryPosgresql) ScenarioWriteRepository {
	return &ScenarioWriteRepositoryPostgresql{
		transactionFactory: transactionFactory,
	}
}

func (repo *ScenarioWriteRepositoryPostgresql) CreateScenario(tx Transaction, organizationId string, scenario models.CreateScenarioInput, newScenarioId string) error {
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

func (repo *ScenarioWriteRepositoryPostgresql) UpdateScenario(tx Transaction, scenario models.UpdateScenarioInput) error {
	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(tx)

	sql := NewQueryBuilder().
		Update(dbmodels.TABLE_SCENARIOS).
		Where("id = ?", scenario.Id)

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

func (repo *ScenarioWriteRepositoryPostgresql) UpdateScenarioLiveIterationId(tx Transaction, scenarioId string, scenarioIterationId *string) error {
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
