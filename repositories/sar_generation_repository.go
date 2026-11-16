package repositories

import (
	"context"
	"encoding/json"
	"time"

	"github.com/Masterminds/squirrel"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories/dbmodels"
	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// nullIfEmpty maps "" to SQL NULL.
func nullIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// rawOrNull maps an empty payload to SQL NULL.
func rawOrNull(b []byte) any {
	if len(b) == 0 {
		return nil
	}
	return json.RawMessage(b)
}

// CreateSarGenerationJob inserts a job. A second pending/running job for the
// same declaration violates the partial unique index and returns
// models.ErrSarJobAlreadyActive.
func (repo *MarbleDbRepository) CreateSarGenerationJob(
	ctx context.Context, exec Executor, j models.SarGenerationJob,
) error {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return err
	}
	query := NewQueryBuilder().
		Insert(dbmodels.TABLE_SAR_GENERATION_JOBS).
		Columns("id", "declaration_id", "kind", "status", "input", "error", "created_by").
		Values(j.Id, j.DeclarationId, string(j.Kind), string(j.Status),
			rawOrNull(j.Input), nullIfEmpty(j.Error), j.CreatedBy)
	err := ExecBuilder(ctx, exec, query)
	if IsUniqueViolationError(err) {
		return models.ErrSarJobAlreadyActive
	}
	return err
}

func (repo *MarbleDbRepository) GetSarGenerationJob(
	ctx context.Context, exec Executor, id uuid.UUID,
) (models.SarGenerationJob, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return models.SarGenerationJob{}, err
	}
	query := NewQueryBuilder().
		Select(dbmodels.SarGenerationJobColumns...).
		From(dbmodels.TABLE_SAR_GENERATION_JOBS).
		Where(squirrel.Eq{"id": id})
	return SqlToModel(ctx, exec, query, dbmodels.AdaptSarGenerationJob)
}

// SetSarGenerationJobStatus updates status and error (empty = no error) and
// bumps updated_at.
func (repo *MarbleDbRepository) SetSarGenerationJobStatus(
	ctx context.Context, exec Executor, id uuid.UUID, status models.SarJobStatus, errMsg string,
) error {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return err
	}
	query := NewQueryBuilder().
		Update(dbmodels.TABLE_SAR_GENERATION_JOBS).
		Set("status", string(status)).
		Set("error", nullIfEmpty(errMsg)).
		Set("updated_at", squirrel.Expr("now()")).
		Where(squirrel.Eq{"id": id})
	err := ExecBuilder(ctx, exec, query)
	if IsUniqueViolationError(err) {
		return models.ErrSarJobAlreadyActive
	}
	return err
}

// TouchSarGenerationJob bumps updated_at without changing anything else. The
// orchestrator calls it as a heartbeat while it works, so that a job stuck in
// "running" with no update for longer than a few task timeouts can be told
// apart from a live one.
func (repo *MarbleDbRepository) TouchSarGenerationJob(
	ctx context.Context, exec Executor, id uuid.UUID,
) error {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return err
	}
	query := NewQueryBuilder().
		Update(dbmodels.TABLE_SAR_GENERATION_JOBS).
		Set("updated_at", squirrel.Expr("now()")).
		Where(squirrel.Eq{"id": id})
	return ExecBuilder(ctx, exec, query)
}

func (repo *MarbleDbRepository) CreateSarGenerationTasks(
	ctx context.Context, exec Executor, tasks []models.SarGenerationTask,
) error {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return err
	}
	if len(tasks) == 0 {
		return nil
	}
	query := NewQueryBuilder().
		Insert(dbmodels.TABLE_SAR_GENERATION_TASKS).
		Columns("id", "job_id", "kind", "target_path", "input", "input_hash", "depends_on", "status")
	for _, t := range tasks {
		dependsOn := t.DependsOn
		if dependsOn == nil {
			dependsOn = []uuid.UUID{}
		}
		query = query.Values(t.Id, t.JobId, t.Kind, nullIfEmpty(t.TargetPath),
			json.RawMessage(t.Input), t.InputHash, dependsOn, string(t.Status))
	}
	return ExecBuilder(ctx, exec, query)
}

// ListSarGenerationTasks returns the tasks of a job in creation order, without
// their trace (see GetSarGenerationTaskTrace).
func (repo *MarbleDbRepository) ListSarGenerationTasks(
	ctx context.Context, exec Executor, jobId uuid.UUID,
) ([]models.SarGenerationTask, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, err
	}
	query := NewQueryBuilder().
		Select(dbmodels.SarGenerationTaskColumns...).
		From(dbmodels.TABLE_SAR_GENERATION_TASKS).
		Where(squirrel.Eq{"job_id": jobId}).
		OrderBy("created_at ASC", "id ASC")
	return SqlToListOfModels(ctx, exec, query, dbmodels.AdaptSarGenerationTask)
}

func (repo *MarbleDbRepository) GetSarGenerationTaskTrace(
	ctx context.Context, exec Executor, id uuid.UUID,
) ([]byte, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, err
	}
	sql, args, err := NewQueryBuilder().
		Select("trace").
		From(dbmodels.TABLE_SAR_GENERATION_TASKS).
		Where(squirrel.Eq{"id": id}).
		ToSql()
	if err != nil {
		return nil, errors.Wrap(err, "cannot build sar task trace query")
	}
	var trace []byte
	if err := exec.QueryRow(ctx, sql, args...).Scan(&trace); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, models.NotFoundError
		}
		return nil, errors.Wrap(err, "cannot read sar task trace")
	}
	return trace, nil
}

// ClaimSarGenerationTask atomically moves a task to running and increments its
// attempts. It succeeds only if the task is pending or failed, or running but
// started before staleBefore (its worker is presumed dead). It returns false
// when the task is not claimable (done, or running and fresh), so a task is
// never executed by two workers nor re-executed once done.
func (repo *MarbleDbRepository) ClaimSarGenerationTask(
	ctx context.Context, exec Executor, id uuid.UUID, staleBefore time.Time,
) (bool, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return false, err
	}
	sql, args, err := NewQueryBuilder().
		Update(dbmodels.TABLE_SAR_GENERATION_TASKS).
		Set("status", string(models.SarJobStatusRunning)).
		Set("attempts", squirrel.Expr("attempts + 1")).
		Set("started_at", squirrel.Expr("now()")).
		Set("error", nil).
		Where(squirrel.Eq{"id": id}).
		Where(squirrel.Or{
			squirrel.Eq{"status": []string{
				string(models.SarJobStatusPending), string(models.SarJobStatusFailed),
			}},
			squirrel.And{
				squirrel.Eq{"status": string(models.SarJobStatusRunning)},
				squirrel.Lt{"started_at": staleBefore},
			},
		}).
		Suffix("RETURNING id").
		ToSql()
	if err != nil {
		return false, errors.Wrap(err, "cannot build sar task claim query")
	}
	rows, err := exec.Query(ctx, sql, args...)
	if err != nil {
		return false, errors.Wrap(err, "cannot claim sar task")
	}
	defer rows.Close()
	claimed := rows.Next()
	if err := rows.Err(); err != nil {
		return false, errors.Wrap(err, "cannot claim sar task")
	}
	return claimed, nil
}

func (repo *MarbleDbRepository) CompleteSarGenerationTask(
	ctx context.Context, exec Executor, id uuid.UUID, output []byte, trace []byte,
) error {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return err
	}
	query := NewQueryBuilder().
		Update(dbmodels.TABLE_SAR_GENERATION_TASKS).
		Set("status", string(models.SarJobStatusDone)).
		Set("output", rawOrNull(output)).
		Set("trace", rawOrNull(trace)).
		Set("finished_at", squirrel.Expr("now()")).
		Where(squirrel.Eq{"id": id})
	return ExecBuilder(ctx, exec, query)
}

func (repo *MarbleDbRepository) FailSarGenerationTask(
	ctx context.Context, exec Executor, id uuid.UUID, errMsg string, trace []byte,
) error {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return err
	}
	query := NewQueryBuilder().
		Update(dbmodels.TABLE_SAR_GENERATION_TASKS).
		Set("status", string(models.SarJobStatusFailed)).
		Set("error", nullIfEmpty(errMsg)).
		Set("trace", rawOrNull(trace)).
		Set("finished_at", squirrel.Expr("now()")).
		Where(squirrel.Eq{"id": id})
	return ExecBuilder(ctx, exec, query)
}

func (repo *MarbleDbRepository) CreateSarQuestions(
	ctx context.Context, exec Executor, qs []models.SarQuestion,
) error {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return err
	}
	if len(qs) == 0 {
		return nil
	}
	query := NewQueryBuilder().
		Insert(dbmodels.TABLE_SAR_DECLARATION_QUESTION).
		Columns("id", "declaration_id", "task_id", "kind", "severity", "paths", "text", "options", "status")
	for _, q := range qs {
		paths := q.Paths
		if paths == nil {
			paths = []string{}
		}
		query = query.Values(q.Id, q.DeclarationId, q.TaskId, q.Kind, nullIfEmpty(q.Severity),
			paths, q.Text, rawOrNull(q.Options), q.Status)
	}
	return ExecBuilder(ctx, exec, query)
}

// ListSarQuestions lists the questions of a declaration, oldest first. An
// empty status returns all of them.
func (repo *MarbleDbRepository) ListSarQuestions(
	ctx context.Context, exec Executor, declarationId uuid.UUID, status string,
) ([]models.SarQuestion, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, err
	}
	query := NewQueryBuilder().
		Select(dbmodels.SarQuestionColumns...).
		From(dbmodels.TABLE_SAR_DECLARATION_QUESTION).
		Where(squirrel.Eq{"declaration_id": declarationId}).
		OrderBy("created_at ASC", "id ASC")
	if status != "" {
		query = query.Where(squirrel.Eq{"status": status})
	}
	return SqlToListOfModels(ctx, exec, query, dbmodels.AdaptSarQuestion)
}

func (repo *MarbleDbRepository) GetSarQuestion(
	ctx context.Context, exec Executor, id uuid.UUID,
) (models.SarQuestion, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return models.SarQuestion{}, err
	}
	query := NewQueryBuilder().
		Select(dbmodels.SarQuestionColumns...).
		From(dbmodels.TABLE_SAR_DECLARATION_QUESTION).
		Where(squirrel.Eq{"id": id})
	return SqlToModel(ctx, exec, query, dbmodels.AdaptSarQuestion)
}

// CloseSarQuestions sets the status (resolved or dismissed) and closed_at.
func (repo *MarbleDbRepository) CloseSarQuestions(
	ctx context.Context, exec Executor, ids []uuid.UUID, status string,
) error {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return err
	}
	if len(ids) == 0 {
		return nil
	}
	query := NewQueryBuilder().
		Update(dbmodels.TABLE_SAR_DECLARATION_QUESTION).
		Set("status", status).
		Set("closed_at", squirrel.Expr("now()")).
		Where(squirrel.Eq{"id": ids})
	return ExecBuilder(ctx, exec, query)
}
