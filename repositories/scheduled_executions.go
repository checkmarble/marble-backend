package repositories

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Masterminds/squirrel"
	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/repositories/dbmodels"
	"github.com/checkmarble/marble-backend/utils"
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
		query = query.Limit(uint64(paging.Limit))

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

	if filters.OrganizationId != uuid.Nil {
		query = query.Where(squirrel.Eq{"se.organization_id": filters.OrganizationId})
	}
	if filters.ScenarioId != "" {
		query = query.Where(squirrel.Eq{"se.scenario_id": filters.ScenarioId})
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
				"deduplicate_objects",
			).
			Values(
				newScheduledExecutionId,
				createScheduledEx.OrganizationId,
				createScheduledEx.ScenarioId,
				createScheduledEx.ScenarioIterationId,
				models.ScheduledExecutionPending.String(),
				createScheduledEx.Manual,
				// Defensive default: callers are expected to have already resolved this to
				// a definite value (see CreateScheduledExecutionInput.DeduplicateObjects).
				utils.Or(createScheduledEx.DeduplicateObjects, false),
			),
	)
	return err
}

// UpdateScheduledExecutionStatus transitions the execution to a new status, using the current
// status as an optimistic lock: by default, only pending or processing executions can be updated,
// so that a worker can never overwrite a final status or a requested cancellation. A stricter
// expected status can be given with FromStatus. Returns whether the row was updated.
func (repo *MarbleDbRepository) UpdateScheduledExecutionStatus(
	ctx context.Context,
	exec Executor,
	updateScheduledEx models.UpdateScheduledExecutionStatusInput,
) (bool, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return false, err
	}
	query := NewQueryBuilder().
		Update(dbmodels.TABLE_SCHEDULED_EXECUTIONS).
		Where("id = ?", updateScheduledEx.Id)

	if updateScheduledEx.FromStatus != nil {
		query = query.Where("status = ?", updateScheduledEx.FromStatus.String())
	} else {
		query = query.Where(squirrel.Eq{"status": []string{
			models.ScheduledExecutionPending.String(),
			models.ScheduledExecutionProcessing.String(),
		}})
	}

	query = query.Set("status", updateScheduledEx.Status.String())
	if updateScheduledEx.Status == models.ScheduledExecutionSuccess {
		query = query.Set("finished_at", squirrel.Expr("NOW()"))
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
	return tag.RowsAffected() > 0, nil
}

// RequestScheduledExecutionCancel moves an execution that is currently in fromStatus to the
// cancelling state. Returns false if the execution was not in fromStatus anymore.
func (repo *MarbleDbRepository) RequestScheduledExecutionCancel(
	ctx context.Context,
	exec Executor,
	id string,
	fromStatus models.ScheduledExecutionStatus,
	cancelDeadline time.Time,
) (bool, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return false, err
	}

	sql, args, err := NewQueryBuilder().
		Update(dbmodels.TABLE_SCHEDULED_EXECUTIONS).
		Set("status", models.ScheduledExecutionCancelling.String()).
		Set("cancel_requested_at", squirrel.Expr("NOW()")).
		Set("cancel_deadline", cancelDeadline).
		Where("id = ?", id).
		Where("status = ?", fromStatus.String()).
		ToSql()

	if err != nil {
		return false, err
	}

	tag, err := exec.Exec(ctx, sql, args...)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}

// AckScheduledExecutionCancel is called by the worker owning a cancelling execution once it has
// stopped working on it, to mark it as cancelled.
func (repo *MarbleDbRepository) AckScheduledExecutionCancel(ctx context.Context, exec Executor, id string) error {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return err
	}

	return ExecBuilder(ctx, exec, NewQueryBuilder().
		Update(dbmodels.TABLE_SCHEDULED_EXECUTIONS).
		Set("status", models.ScheduledExecutionCancelled.String()).
		Set("finished_at", squirrel.Expr("NOW()")).
		Where("id = ?", id).
		Where("status = ?", models.ScheduledExecutionCancelling.String()))
}

// FinalizeExpiredCancellations marks as cancelled the cancelling executions whose cancel deadline
// has passed, meaning the worker that owned them cannot be running anymore. At least one of the
// filters must be provided.
//
// The deadline is compared with the application clock, not the database's: it is computed with
// it, and reads already report expired cancellations as cancelled using it (see
// models.EffectiveScheduledExecutionStatus), so both must agree on whether it has passed.
func (repo *MarbleDbRepository) FinalizeExpiredCancellations(ctx context.Context, exec Executor, filters models.FinalizeExpiredCancellationsFilters) error {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return err
	}
	if filters.ScheduledExecutionId == "" && filters.ScenarioId == "" {
		return errors.New("FinalizeExpiredCancellations requires a scheduled execution or scenario id")
	}

	query := NewQueryBuilder().
		Update(dbmodels.TABLE_SCHEDULED_EXECUTIONS).
		Set("status", models.ScheduledExecutionCancelled.String()).
		Set("finished_at", squirrel.Expr("NOW()")).
		Where("status = ?", models.ScheduledExecutionCancelling.String()).
		Where("cancel_deadline < ?", time.Now())

	if filters.ScheduledExecutionId != "" {
		query = query.Where("id = ?", filters.ScheduledExecutionId)
	}
	if filters.ScenarioId != "" {
		query = query.Where("scenario_id = ?", filters.ScenarioId)
	}

	return ExecBuilder(ctx, exec, query)
}

func (repo *MarbleDbRepository) UpdateScheduledExecution(ctx context.Context, exec Executor, input models.UpdateScheduledExecutionInput) error {
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
	if input.ManifestBlobKey != nil {
		query = query.Set("manifest_blob_key", *input.ManifestBlobKey)
	}
	if input.Deadline != nil {
		query = query.Set("deadline", *input.Deadline)
	}

	return ExecBuilder(ctx, exec, query)
}

// AdvanceScheduledExecutionManifest moves the v2 coordinator's manifest cursor and
// progress counters forward. It is meant to run in the same transaction as the batch's
// decision inserts so the offset and the persisted decisions commit together. It returns
// models.ErrScheduledExecutionNotProcessing if the execution is not processing anymore.
func (repo *MarbleDbRepository) AdvanceScheduledExecutionManifest(
	ctx context.Context,
	exec Executor,
	input models.AdvanceScheduledExecutionManifestInput,
) error {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return err
	}

	query := NewQueryBuilder().
		Update(dbmodels.TABLE_SCHEDULED_EXECUTIONS).
		Set("manifest_byte_offset", input.ManifestByteOffset).
		Set("manifest_rows_processed", input.ManifestRowsProcessed).
		Set("number_of_created_decisions", input.NumberOfCreatedDecisions).
		Set("number_of_evaluated_decisions", input.NumberOfEvaluatedDecisions).
		Where("id = ?", input.Id).
		Where("status = ?", models.ScheduledExecutionProcessing.String())

	sql, args, err := query.ToSql()
	if err != nil {
		return err
	}

	tag, err := exec.Exec(ctx, sql, args...)
	if err != nil {
		return err
	}
	// The status check makes a batch and a concurrent cancellation mutually exclusive: either the
	// batch commits before the cancellation, or its transaction is rolled back.
	if tag.RowsAffected() == 0 {
		return models.ErrScheduledExecutionNotProcessing
	}
	return nil
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
