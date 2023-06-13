package repositories

import (
	"marble/marble-backend/models"
	"marble/marble-backend/pg_repository"
	"marble/marble-backend/repositories/dbmodels"

	"github.com/Masterminds/squirrel"
)

type ScheduledExecutionRepository interface {
	GetScheduledExecution(tx Transaction, organizationId, id string) (models.ScheduledExecution, error)
	ListScheduledExecutions(tx Transaction, organizationId, scenarioId string) ([]models.ScheduledExecution, error)
	CreateScheduledExecution(tx Transaction, input models.CreateScheduledExecutionInput) error
	UpdateScheduledExecution(tx Transaction, updateScheduledEx models.UpdateScheduledExecutionInput) error
}

type ScheduledExecutionRepositoryPostgresql struct {
	transactionFactory TransactionFactory
	queryBuilder       squirrel.StatementBuilderType
}

func (repo *ScheduledExecutionRepositoryPostgresql) GetScheduledExecution(tx Transaction, organizationId, id string) (models.ScheduledExecution, error) {
	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(tx)
	return SqlToModel(
		pgTx,
		repo.queryBuilder.
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
		repo.queryBuilder.
			Select(dbmodels.ScheduledExecutionFields...).
			From(dbmodels.TABLE_SCHEDULED_EXECUTIONS).
			Where(squirrel.Eq{"organization_id": organizationId}).
			Where(squirrel.Eq{"scenario_id": scenarioId}).
			OrderBy("started_at DESC"),
		dbmodels.AdaptScheduledExecution,
	)
}

func (repo *ScheduledExecutionRepositoryPostgresql) CreateScheduledExecution(tx Transaction, createScheduledEx models.CreateScheduledExecutionInput) error {
	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(tx)

	return SqlInsert(
		pgTx,
		repo.queryBuilder.Insert(dbmodels.TABLE_SCHEDULED_EXECUTIONS).
			Columns(
				"organization_id",
				"scenario_id",
				"scenario_iteration_id",
				"status",
			).
			Values(
				createScheduledEx.Organizationid,
				createScheduledEx.ScenarioId,
				createScheduledEx.ScenarioIterationID,
				"in_progress",
			),
	)
}

func (repo *ScheduledExecutionRepositoryPostgresql) UpdateScheduledExecution(tx Transaction, updateScheduledEx models.UpdateScheduledExecutionInput) error {
	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(tx)
	return SqlUpdate(
		pgTx,
		repo.queryBuilder.
			Update(dbmodels.TABLE_SCHEDULED_EXECUTIONS).
			SetMap(pg_repository.ColumnValueMap(dbmodels.AdaptUpdateScheduledExecutionDbInput(updateScheduledEx))).
			Where("id = ?", updateScheduledEx.ID),
	)
}
