package repositories

import (
	"context"
	"fmt"

	"github.com/Masterminds/squirrel"
	"github.com/jackc/pgx/v5"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories/dbmodels"
)

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

func (repo *MarbleDbRepository) GetScheduledExecution(ctx context.Context, exec Executor, id string) (models.ScheduledExecution, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return models.ScheduledExecution{}, err
	}

	return SqlToRow(
		ctx,
		exec,
		selectJoinScheduledExecutionAndScenario().Where(squirrel.Eq{"se.id": id}),
		adaptJoinScheduledExecutionWithScenario,
	)
}

func (repo *MarbleDbRepository) ListScheduledExecutions(ctx context.Context, exec Executor,
	filters models.ListScheduledExecutionsFilters,
) ([]models.ScheduledExecution, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, err
	}
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
		ctx,
		exec,
		query,
		adaptJoinScheduledExecutionWithScenario,
	)
}

func (repo *MarbleDbRepository) CreateScheduledExecution(ctx context.Context, exec Executor,
	createScheduledEx models.CreateScheduledExecutionInput, newScheduledExecutionId string,
) error {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return err
	}

	err := ExecBuilder(
		ctx,
		exec,
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

func (repo *MarbleDbRepository) UpdateScheduledExecutionStatus(
	ctx context.Context,
	exec Executor,
	updateScheduledEx models.UpdateScheduledExecutionStatusInput,
) (executed bool, err error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return false, err
	}
	query := NewQueryBuilder().
		Update(dbmodels.TABLE_SCHEDULED_EXECUTIONS).
		Where("id = ?", updateScheduledEx.Id).
		Where("status = ?", updateScheduledEx.CurrentStatusCondition.String())

	query = query.Set("status", updateScheduledEx.Status.String())
	if updateScheduledEx.Status == models.ScheduledExecutionSuccess {
		query = query.Set("finished_at", "NOW()")
	}

	if updateScheduledEx.NumberOfCreatedDecisions != nil {
		query = query.Set("number_of_created_decisions",
			*updateScheduledEx.NumberOfCreatedDecisions)
	}

	// return ExecBuilder(ctx, exec, query)
	sql, args, err := query.ToSql()
	if err != nil {
		return false, err
	}

	tag, err := exec.Exec(ctx, sql, args...)
	if err != nil {
		return false, err
	}
	if tag.RowsAffected() == 0 {
		return false, nil
	}
	return true, nil
}
