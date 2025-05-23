package repositories

import (
	"context"
	"fmt"
	"strings"

	"github.com/Masterminds/squirrel"
	"github.com/jackc/pgx/v5"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/repositories/dbmodels"
)

type dbJoinScheduledExecutionAndScenario struct {
	dbmodels.DBScheduledExecution
	dbmodels.DBScenario
	dbmodels.DBScenarioIteration
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
	iteration, err := dbmodels.AdaptScenarioIteration(db.DBScenarioIteration)
	if err != nil {
		return models.ScheduledExecution{}, err
	}
	return dbmodels.AdaptScheduledExecution(db.DBScheduledExecution, scenario, iteration), nil
}

func selectJoinScheduledExecutionAndScenario() squirrel.SelectBuilder {
	var columns []string
	columns = append(columns, columnsNames("se", dbmodels.ScheduledExecutionFields)...)
	columns = append(columns, columnsNames("scenario", dbmodels.SelectScenarioColumn)...)
	columns = append(columns, columnsNames("scenario_iterations",
		dbmodels.SelectScenarioIterationColumn)...)

	return NewQueryBuilder().
		Select(columns...).
		From(fmt.Sprintf("%s AS se", dbmodels.TABLE_SCHEDULED_EXECUTIONS)).
		Join(fmt.Sprintf("%s AS scenario ON scenario.id = se.scenario_id", dbmodels.TABLE_SCENARIOS)).
		Join(fmt.Sprintf("%s AS scenario_iterations ON scenario_iterations.id = se.scenario_iteration_id", dbmodels.TABLE_SCENARIO_ITERATIONS))
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
	filters models.ListScheduledExecutionsFilters, paging *models.PaginationAndSorting,
) ([]models.ScheduledExecution, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, err
	}

	query := selectJoinScheduledExecutionAndScenario().OrderBy("se.started_at DESC, se.id DESC")

	if paging != nil {
		query = query.Limit(uint64(paging.Limit + 1))

		if paging.OffsetId != "" {
			switch cursorExecution, err := repo.GetScheduledExecution(ctx, exec, paging.OffsetId); err {
			case nil:
				query = query.Where(squirrel.Expr("(se.started_at, se.id) < (?, ?)",
					cursorExecution.StartedAt, cursorExecution.FinishedAt))
			default:
				return nil, err
			}
		}
	}

	if filters.OrganizationId != "" {
		query = query.Where(squirrel.Eq{"se.organization_id": filters.OrganizationId})
	}
	if filters.ScenarioId != "" {
		query = query.Where(squirrel.Eq{"se.scenario_id": filters.ScenarioId})
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
	// uses optimistic locking based on the current status to avoid overwriting the status incorrectly
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

	if updateScheduledEx.NumberOfEvaluatedDecisions != nil {
		query = query.Set("number_of_evaluated_decisions",
			*updateScheduledEx.NumberOfEvaluatedDecisions)
	}

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

func (repo *MarbleDbRepository) UpdateScheduledExecution(
	ctx context.Context,
	exec Executor,
	input models.UpdateScheduledExecutionInput,
) error {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return err
	}
	query := NewQueryBuilder().
		Update(dbmodels.TABLE_SCHEDULED_EXECUTIONS).
		Where("id = ?", input.Id)

	if input.NumberOfPlannedDecisions != nil {
		query = query.
			Set("number_of_planned_decisions", *input.NumberOfPlannedDecisions)
	}

	return ExecBuilder(ctx, exec, query)
}

func (repo *MarbleDbRepository) StoreDecisionsToCreate(
	ctx context.Context,
	exec Executor,
	decisionsToCreate models.DecisionToCreateBatchCreateInput,
) ([]models.DecisionToCreate, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, err
	}

	if len(decisionsToCreate.ObjectId) == 0 {
		return nil, nil
	}

	query := NewQueryBuilder().
		Insert(dbmodels.TABLE_DECISIONS_TO_CREATE).
		Columns(
			"scheduled_execution_id",
			"object_id",
		)

	for _, objectId := range decisionsToCreate.ObjectId {
		query = query.Values(
			decisionsToCreate.ScheduledExecutionId,
			objectId,
		)
	}

	query = query.Suffix(fmt.Sprintf("RETURNING %s",
		strings.Join(dbmodels.DecisionToCreateFields, ",")))

	// the query can be quite large (65k uuids ~= 2Mb), so avoid to passing it by value to the utility functions
	sql, args, err := query.ToSql()
	if err != nil {
		return nil, err
	}

	rows, err := exec.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}

	var decisions []models.DecisionToCreate
	for rows.Next() {
		db, err := pgx.RowToStructByPos[dbmodels.DecisionToCreate](rows)
		if err != nil {
			return nil, err
		}
		models, err := dbmodels.AdaptDecisionToCreate(db)
		if err != nil {
			return nil, err
		}
		decisions = append(decisions, models)
	}

	err = rows.Err()
	if err != nil {
		return nil, err
	}

	return decisions, nil
}

func (repo *MarbleDbRepository) UpdateDecisionToCreateStatus(
	ctx context.Context,
	exec Executor,
	id string,
	status models.DecisionToCreateStatus,
) error {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return err
	}

	query := NewQueryBuilder().
		Update(dbmodels.TABLE_DECISIONS_TO_CREATE).
		Where("id = ?", id).
		Set("status", status).
		Set("updated_at", "NOW()")

	return ExecBuilder(ctx, exec, query)
}

func (repo *MarbleDbRepository) GetDecisionToCreate(
	ctx context.Context,
	exec Executor,
	decisionToCreateId string,
	forUpdate ...bool,
) (models.DecisionToCreate, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return models.DecisionToCreate{}, err
	}

	query := NewQueryBuilder().
		Select(dbmodels.DecisionToCreateFields...).
		From(dbmodels.TABLE_DECISIONS_TO_CREATE).
		Where("id = ?", decisionToCreateId)

	if len(forUpdate) > 0 && forUpdate[0] {
		query = query.Suffix("FOR UPDATE")
	}

	return SqlToModel(ctx, exec, query, dbmodels.AdaptDecisionToCreate)
}

func (repo *MarbleDbRepository) ListDecisionsToCreate(
	ctx context.Context,
	exec Executor,
	filters models.ListDecisionsToCreateFilters,
	limit *int,
) ([]models.DecisionToCreate, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, err
	}

	query := NewQueryBuilder().
		Select(dbmodels.DecisionToCreateFields...).
		From(dbmodels.TABLE_DECISIONS_TO_CREATE).
		Where("scheduled_execution_id = ?", filters.ScheduledExecutionId)

	if len(filters.Status) > 0 {
		query = query.Where(squirrel.Eq{
			"status": pure_utils.Map(
				filters.Status,
				func(s models.DecisionToCreateStatus) string { return string(s) },
			),
		})
	}

	if limit != nil {
		query = query.Limit(uint64(*limit))
	}

	return SqlToListOfModels(ctx, exec, query, dbmodels.AdaptDecisionToCreate)
}

func (repo *MarbleDbRepository) CountCompletedDecisionsByStatus(
	ctx context.Context,
	exec Executor,
	scheduledExecutionId string,
) (models.DecisionToCreateCountMetadata, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return models.DecisionToCreateCountMetadata{}, err
	}

	query := NewQueryBuilder().
		Select("status, COUNT(*) AS c").
		From(dbmodels.TABLE_DECISIONS_TO_CREATE).
		Where("scheduled_execution_id = ?", scheduledExecutionId).
		Where("status IN (?, ?)",
			models.DecisionToCreateStatusCreated, models.DecisionToCreateStatusTriggerConditionMismatch).
		GroupBy("status")

	sql, args, err := query.ToSql()
	if err != nil {
		return models.DecisionToCreateCountMetadata{}, err
	}

	rows, err := exec.Query(ctx, sql, args...)
	if err != nil {
		return models.DecisionToCreateCountMetadata{}, err
	}

	counts := models.DecisionToCreateCountMetadata{}
	for rows.Next() {
		var status string
		var count int
		err = rows.Scan(&status, &count)
		if err != nil {
			return models.DecisionToCreateCountMetadata{}, err
		}

		switch status {
		case string(models.DecisionToCreateStatusCreated):
			counts.Created = count
		case string(models.DecisionToCreateStatusTriggerConditionMismatch):
			counts.TriggerConditionMismatch = count
		}
	}

	err = rows.Err()
	if err != nil {
		return models.DecisionToCreateCountMetadata{}, err
	}

	counts.SuccessfullyEvaluated = counts.Created + counts.TriggerConditionMismatch

	return counts, nil
}
