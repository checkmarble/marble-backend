package repositories

import (
	"fmt"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories/dbmodels"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/jackc/pgx/v5"

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

var joinScheduledExecutionAndScenarioFields = func() []string {
	var columns []string
	columns = append(columns, columnsNames("se", dbmodels.ScheduledExecutionFields)...)
	columns = append(columns, columnsNames("scenario", dbmodels.SelectScenarioColumn)...)
	return columns
}()

func adaptJoinScheduledExecutionWithScenario(row pgx.CollectableRow) (models.ScheduledExecution, error) {

	db, err := pgx.RowToStructByPos[dbJoinScheduledExecutionAndScenario](row)
	if err != nil {
		return models.ScheduledExecution{}, err
	}

	scenario := dbmodels.AdaptScenario(db.DBScenario)
	return dbmodels.AdaptScheduledExecution(db.DBScheduledExecution, scenario), nil
}

func selectJoinScheduledExecutionAndScenario() squirrel.SelectBuilder {
	return NewQueryBuilder().
		Select(joinScheduledExecutionAndScenarioFields...).
		From(fmt.Sprintf("%s AS se", dbmodels.TABLE_SCHEDULED_EXECUTIONS)).
		LeftJoin(fmt.Sprintf("%s AS scenario ON scenario.id = se.scenario_id", dbmodels.TABLE_SCENARIOS))
}

func (repo *ScheduledExecutionRepositoryPostgresql) GetScheduledExecution(tx Transaction, id string) (models.ScheduledExecution, error) {
	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(tx)

	return SqlToRow(
		pgTx,
		selectJoinScheduledExecutionAndScenario().Where(squirrel.Eq{"se.id": id}),
		adaptJoinScheduledExecutionWithScenario,
	)
}

func (repo *ScheduledExecutionRepositoryPostgresql) ListScheduledExecutionsOfScenario(tx Transaction, scenarioId string) ([]models.ScheduledExecution, error) {

	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(tx)
	return SqlToListOfRow(
		pgTx,
		selectJoinScheduledExecutionAndScenario().
			Where(squirrel.Eq{"se.scenario_id": scenarioId}).
			OrderBy("se.started_at DESC"),
		adaptJoinScheduledExecutionWithScenario,
	)
}

func (repo *ScheduledExecutionRepositoryPostgresql) ListScheduledExecutionsOfOrganization(tx Transaction, organizationId string) ([]models.ScheduledExecution, error) {
	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(tx)

	return SqlToListOfRow(
		pgTx,
		selectJoinScheduledExecutionAndScenario().
			Where(squirrel.Eq{"se.organization_id": organizationId}).
			OrderBy("se.started_at DESC"),
		adaptJoinScheduledExecutionWithScenario,
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

	if updateScheduledEx.NumberOfCreatedDecisions != nil {
		query = query.Set("number_of_created_decisions", *updateScheduledEx.NumberOfCreatedDecisions)
	}

	_, err := pgTx.ExecBuilder(query)
	return err
}
