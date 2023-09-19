package repositories

import (
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories/dbmodels"

	"github.com/Masterminds/squirrel"
)

type ScheduledExecutionRepository interface {
	GetScheduledExecution(tx Transaction, id string) (models.ScheduledExecution, error)
	ListScheduledExecutionsOfScenario(tx Transaction, scenarioId string) ([]models.ScheduledExecution, error)
	ListScheduledExecutionsOfOrganization(tx Transaction, organizationId string) ([]models.ScheduledExecution, error)
	CreateScheduledExecution(tx Transaction, input models.CreateScheduledExecutionInput, newScheduledExecutionId string) error
	UpdateScheduledExecution(tx Transaction, updateScheduledEx models.UpdateScheduledExecutionInput) error
}

type ScheduledExecutionRepositoryPostgresql struct {
	transactionFactory TransactionFactory
}

func (repo *ScheduledExecutionRepositoryPostgresql) GetScheduledExecution(tx Transaction, id string) (models.ScheduledExecution, error) {
	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(tx)
	return SqlToModel(
		pgTx,
		NewQueryBuilder().
			Select(dbmodels.ScheduledExecutionFields...).
			From(dbmodels.TABLE_SCHEDULED_EXECUTIONS).
			Where(squirrel.Eq{"id": id}),
		dbmodels.AdaptScheduledExecution,
	)
}

func (repo *ScheduledExecutionRepositoryPostgresql) ListScheduledExecutionsOfScenario(tx Transaction, scenarioId string) ([]models.ScheduledExecution, error) {
	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(tx)
	return SqlToListOfModels(
		pgTx,
		NewQueryBuilder().
			Select(dbmodels.ScheduledExecutionFields...).
			From(dbmodels.TABLE_SCHEDULED_EXECUTIONS).
			Where(squirrel.Eq{"scenario_id": scenarioId}).
			OrderBy("started_at DESC"),
		dbmodels.AdaptScheduledExecution,
	)
}

func (repo *ScheduledExecutionRepositoryPostgresql) ListScheduledExecutionsOfOrganization(tx Transaction, organizationId string) ([]models.ScheduledExecution, error) {
	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(tx)
	return SqlToListOfModels(
		pgTx,
		NewQueryBuilder().
			Select(dbmodels.ScheduledExecutionFields...).
			From(dbmodels.TABLE_SCHEDULED_EXECUTIONS).
			Where(squirrel.Eq{"organization_id": organizationId}).
			OrderBy("started_at DESC"),
		dbmodels.AdaptScheduledExecution,
	)
}

func (repo *ScheduledExecutionRepositoryPostgresql) CreateScheduledExecution(tx Transaction, createScheduledEx models.CreateScheduledExecutionInput, newScheduledExecutionId string) error {
	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(tx)

	_, err := pgTx.ExecBuilder(
		NewQueryBuilder().Insert(dbmodels.TABLE_SCHEDULED_EXECUTIONS).
			Columns(
				"id",
				"organization_id",
				"scenario_id",
				"scenario_iteration_id",
				"status",
			).
			Values(
				newScheduledExecutionId,
				createScheduledEx.OrganizationId,
				createScheduledEx.ScenarioId,
				createScheduledEx.ScenarioIterationId,
				models.ScheduledExecutionPending.String(),
			),
	)
	return err
}

func (repo *ScheduledExecutionRepositoryPostgresql) UpdateScheduledExecution(tx Transaction, updateScheduledEx models.UpdateScheduledExecutionInput) error {
	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(tx)
	query := NewQueryBuilder().Update(dbmodels.TABLE_SCHEDULED_EXECUTIONS).
		Where("id = ?", updateScheduledEx.Id)

	if updateScheduledEx.Status != nil {
		query = query.Set("status", updateScheduledEx.Status.String())
		if *updateScheduledEx.Status == models.ScheduledExecutionSuccess {
			query = query.Set("finished_at", "NOW()")
		}
	}

	_, err := pgTx.ExecBuilder(query)
	return err
}
