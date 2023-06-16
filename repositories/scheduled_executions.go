package repositories

import (
	"marble/marble-backend/models"
	"marble/marble-backend/repositories/dbmodels"

	"github.com/Masterminds/squirrel"
)

type ScheduledExecutionRepository interface {
	GetScheduledExecution(tx Transaction, organizationId, id string) (models.ScheduledExecution, error)
	ListScheduledExecutions(tx Transaction, organizationId, scenarioId string) ([]models.ScheduledExecution, error)
	CreateScheduledExecution(tx Transaction, input models.CreateScheduledExecutionInput, newScheduledExecutionId string) error
	UpdateScheduledExecution(tx Transaction, updateScheduledEx models.UpdateScheduledExecutionInput) error
}

type ScheduledExecutionRepositoryPostgresql struct {
	transactionFactory TransactionFactory
}

func (repo *ScheduledExecutionRepositoryPostgresql) GetScheduledExecution(tx Transaction, organizationId, id string) (models.ScheduledExecution, error) {
	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(tx)
	return SqlToModel(
		pgTx,
		NewQueryBuilder().
			Select(dbmodels.ScheduledExecutionFields...).
			From(dbmodels.TABLE_SCHEDULED_EXECUTIONS).
			Where(squirrel.Eq{"organization_id": organizationId}).
			Where(squirrel.Eq{"id": id}),
		dbmodels.AdaptScheduledExecution,
	)
}

func (repo *ScheduledExecutionRepositoryPostgresql) ListScheduledExecutions(tx Transaction, organizationId, scenarioId string) ([]models.ScheduledExecution, error) {
	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(tx)
	return SqlToListOfModels(
		pgTx,
		NewQueryBuilder().
			Select(dbmodels.ScheduledExecutionFields...).
			From(dbmodels.TABLE_SCHEDULED_EXECUTIONS).
			Where(squirrel.Eq{"organization_id": organizationId}).
			Where(squirrel.Eq{"scenario_id": scenarioId}).
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
				createScheduledEx.OrganizationID,
				createScheduledEx.ScenarioID,
				createScheduledEx.ScenarioIterationID,
				models.ScheduledExecutionPending.String(),
			),
	)
	return err
}

func (repo *ScheduledExecutionRepositoryPostgresql) UpdateScheduledExecution(tx Transaction, updateScheduledEx models.UpdateScheduledExecutionInput) error {
	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(tx)
	query := NewQueryBuilder().Update(dbmodels.TABLE_SCHEDULED_EXECUTIONS).
		Where("id = ?", updateScheduledEx.ID)

	if updateScheduledEx.Status != nil {
		query = query.Set("status", updateScheduledEx.Status.String())
		if *updateScheduledEx.Status == models.ScheduledExecutionSuccess {
			query = query.Set("finished_at", "NOW()")
		}
	}

	_, err := pgTx.ExecBuilder(query)
	return err
}
