package repositories

import (
	"context"
	"time"

	"github.com/Masterminds/squirrel"
	"github.com/cockroachdb/errors"
	"github.com/google/uuid"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories/dbmodels"
)

type AsyncDecisionExecutionRepository interface {
	CreateAsyncDecisionExecution(ctx context.Context, tx Transaction,
		input models.AsyncDecisionExecutionCreate) error
	CreateAsyncDecisionExecutionBatch(ctx context.Context, tx Transaction,
		inputs []models.AsyncDecisionExecutionCreate) error
	GetAsyncDecisionExecution(ctx context.Context, exec Executor, id uuid.UUID) (models.AsyncDecisionExecution, error)
	UpdateAsyncDecisionExecution(ctx context.Context, exec Executor,
		input models.AsyncDecisionExecutionUpdate) error
	DeleteOldAsyncDecisionExecutionsBatch(ctx context.Context, exec Executor, olderThan time.Time, limit int) (int64, error)
}

func (repo *MarbleDbRepository) CreateAsyncDecisionExecution(
	ctx context.Context,
	tx Transaction,
	input models.AsyncDecisionExecutionCreate,
) error {
	if err := validateMarbleDbExecutor(tx); err != nil {
		return err
	}

	query := NewQueryBuilder().Insert(dbmodels.TABLE_ASYNC_DECISION_EXECUTIONS).
		Columns(
			"id",
			"org_id",
			"object_type",
			"trigger_object",
			"scenario_id",
			"should_ingest",
		).
		Values(
			input.Id,
			input.OrgId,
			input.ObjectType,
			input.TriggerObject,
			input.ScenarioId,
			input.ShouldIngest,
		)

	sql, args, err := query.ToSql()
	if err != nil {
		return errors.Wrap(err, "error building query for CreateAsyncDecisionExecution")
	}

	_, err = tx.Exec(ctx, sql, args...)
	if err != nil {
		return errors.Wrap(err, "error creating async decision execution")
	}
	return nil
}

func (repo *MarbleDbRepository) CreateAsyncDecisionExecutionBatch(
	ctx context.Context,
	tx Transaction,
	inputs []models.AsyncDecisionExecutionCreate,
) error {
	if err := validateMarbleDbExecutor(tx); err != nil {
		return err
	}
	if len(inputs) == 0 {
		return nil
	}

	query := NewQueryBuilder().Insert(dbmodels.TABLE_ASYNC_DECISION_EXECUTIONS).
		Columns(
			"id",
			"org_id",
			"object_type",
			"trigger_object",
			"scenario_id",
			"should_ingest",
		)

	for _, input := range inputs {
		query = query.Values(
			input.Id,
			input.OrgId,
			input.ObjectType,
			input.TriggerObject,
			input.ScenarioId,
			input.ShouldIngest,
		)
	}

	sql, args, err := query.ToSql()
	if err != nil {
		return errors.Wrap(err, "error building query for CreateAsyncDecisionExecutionBatch")
	}

	_, err = tx.Exec(ctx, sql, args...)
	if err != nil {
		return errors.Wrap(err, "error batch creating async decision executions")
	}
	return nil
}

func (repo *MarbleDbRepository) GetAsyncDecisionExecution(
	ctx context.Context,
	exec Executor,
	id uuid.UUID,
) (models.AsyncDecisionExecution, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return models.AsyncDecisionExecution{}, err
	}

	return SqlToModel(
		ctx,
		exec,
		NewQueryBuilder().
			Select(dbmodels.SelectAsyncDecisionExecutionColumns...).
			From(dbmodels.TABLE_ASYNC_DECISION_EXECUTIONS).
			Where(squirrel.Eq{"id": id}),
		dbmodels.AdaptAsyncDecisionExecution,
	)
}

func (repo *MarbleDbRepository) UpdateAsyncDecisionExecution(
	ctx context.Context,
	exec Executor,
	input models.AsyncDecisionExecutionUpdate,
) error {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return err
	}

	query := NewQueryBuilder().
		Update(dbmodels.TABLE_ASYNC_DECISION_EXECUTIONS).
		Set("updated_at", squirrel.Expr("NOW()")).
		Where(squirrel.Eq{"id": input.Id})

	if input.Status != nil {
		query = query.Set("status", input.Status.String())
	}
	if input.DecisionIds != nil {
		query = query.Set("decision_ids", *input.DecisionIds)
	}
	if input.ErrorMessage != nil {
		query = query.Set("error_message", *input.ErrorMessage)
	}

	return ExecBuilder(ctx, exec, query)
}

func (repo *MarbleDbRepository) DeleteOldAsyncDecisionExecutionsBatch(
	ctx context.Context,
	exec Executor,
	olderThan time.Time,
	limit int,
) (int64, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return 0, err
	}

	subquery := NewQueryBuilder().
		Select("id").
		From(dbmodels.TABLE_ASYNC_DECISION_EXECUTIONS).
		Where(squirrel.Lt{"created_at": olderThan}).
		Limit(uint64(limit))

	query := NewQueryBuilder().
		Delete(dbmodels.TABLE_ASYNC_DECISION_EXECUTIONS).
		Where(squirrel.Expr("id IN (?)", subquery))

	sql, args, err := query.ToSql()
	if err != nil {
		return 0, errors.Wrap(err, "error building query for DeleteOldAsyncDecisionExecutionsBatch")
	}

	result, err := exec.Exec(ctx, sql, args...)
	if err != nil {
		return 0, errors.Wrap(err, "error deleting old async decision executions")
	}
	return result.RowsAffected(), nil
}
