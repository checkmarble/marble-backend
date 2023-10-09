package repositories

import (
	"fmt"

	"github.com/Masterminds/squirrel"
	"github.com/jackc/pgx/v5"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories/dbmodels"
	"github.com/checkmarble/marble-backend/utils"
)

type ScheduledExecutionRepository interface {
	GetScheduledExecution(tx Transaction, id string) (models.ScheduledExecution, error)
	ListScheduledExecutions(tx Transaction, filters models.ListScheduledExecutionsFilters) ([]models.ScheduledExecution, error)
	CreateScheduledExecution(tx Transaction, input models.CreateScheduledExecutionInput, newScheduledExecutionId string) error
	UpdateScheduledExecution(tx Transaction, updateScheduledEx models.UpdateScheduledExecutionInput) error
}

type ScheduledExecutionRepositoryPostgresql struct {
	transactionFactory TransactionFactoryPosgresql
}

func columnsNames(tablename string, fields []string) []string {
	return utils.Map(fields, func(f string) string {
		return fmt.Sprintf("%s.%s", tablename, f)
	})
}

type dbJoinScheduledExecutionAndScenario struct {
	dbmodels.DBScheduledExecution
	dbmodels.DBScenario
}

func adaptJoinScheduledExecutionWithScenario(row pgx.CollectableRow) (models.ScheduledExecution, error) {

	db, err := pgx.RowToStructByPos[dbJoinScheduledExecutionAndScenario](row)
	if err != nil {
		return models.ScheduledExecution{}, err
	}

	scenario, err := dbmodels.AdaptScenario(db.DBScenario)
	if err != nil {
		return models.ScheduledExecution{}, err
	}
	return dbmodels.AdaptScheduledExecution(db.DBScheduledExecution, scenario), nil
}

func selectJoinScheduledExecutionAndScenario() squirrel.SelectBuilder {
	var columns []string
	columns = append(columns, columnsNames("se", dbmodels.ScheduledExecutionFields)...)
	columns = append(columns, columnsNames("scenario", dbmodels.SelectScenarioColumn)...)

	return NewQueryBuilder().
		Select(columns...).
		From(fmt.Sprintf("%s AS se", dbmodels.TABLE_SCHEDULED_EXECUTIONS)).
		Join(fmt.Sprintf("%s AS scenario ON scenario.id = se.scenario_id", dbmodels.TABLE_SCENARIOS))
}

func (repo *ScheduledExecutionRepositoryPostgresql) GetScheduledExecution(tx Transaction, id string) (models.ScheduledExecution, error) {
	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(tx)

	return SqlToRow(
		pgTx,
		selectJoinScheduledExecutionAndScenario().Where(squirrel.Eq{"se.id": id}),
		adaptJoinScheduledExecutionWithScenario,
	)
}

func (repo *ScheduledExecutionRepositoryPostgresql) ListScheduledExecutions(tx Transaction, filters models.ListScheduledExecutionsFilters) ([]models.ScheduledExecution, error) {
	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(tx)
	query := selectJoinScheduledExecutionAndScenario().OrderBy("se.started_at DESC")

	if filters.ScenarioId != "" {
		query = query.Where(squirrel.Eq{"se.scenario_id": filters.ScenarioId})
	}

	if filters.OrganizationId != "" {
		query = query.Where(squirrel.Eq{"se.organization_id": filters.OrganizationId})
	}

	if filters.Status != nil {
		query = query.Where(squirrel.Eq{"se.status": filters.Status})
	}

	if filters.ExcludeManual {
		query = query.Where(squirrel.NotEq{"se.manual": true})
	}

	return SqlToListOfRow(
		pgTx, query, adaptJoinScheduledExecutionWithScenario,
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
				"manual",
			).
			Values(
				newScheduledExecutionId,
				createScheduledEx.OrganizationId,
				createScheduledEx.ScenarioId,
				createScheduledEx.ScenarioIterationId,
				models.ScheduledExecutionPending.String(),
				createScheduledEx.Manual,
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

	if updateScheduledEx.NumberOfCreatedDecisions != nil {
		query = query.Set("number_of_created_decisions", *updateScheduledEx.NumberOfCreatedDecisions)
	}

	_, err := pgTx.ExecBuilder(query)
	return err
}
