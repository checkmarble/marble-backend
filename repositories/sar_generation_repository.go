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

// sarActiveJobIndex is the partial unique index allowing one active job per declaration.
const sarActiveJobIndex = "sar_generation_jobs_one_active_idx"

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
	if IsUniqueViolationErrorOf(err, sarActiveJobIndex) {
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

// ListSarGenerationJobs returns the jobs of a declaration, newest first.
func (repo *MarbleDbRepository) ListSarGenerationJobs(
	ctx context.Context, exec Executor, declarationId uuid.UUID,
) ([]models.SarGenerationJob, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, err
	}
	query := NewQueryBuilder().
		Select(dbmodels.SarGenerationJobColumns...).
		From(dbmodels.TABLE_SAR_GENERATION_JOBS).
		Where(squirrel.Eq{"declaration_id": declarationId}).
		OrderBy("created_at DESC", "id DESC")
	return SqlToListOfModels(ctx, exec, query, dbmodels.AdaptSarGenerationJob)
}

// SetSarGenerationJobStatus is a guarded transition that bumps updated_at.
// Allowed: pending|running -> running, pending|running -> done|failed (error
// message stored, empty = none). It returns false when the job is not in an
// allowed source status (e.g. already done or failed: the caller must stop).
// Moving a job back to pending is ResumeSarGenerationJob's business.
func (repo *MarbleDbRepository) SetSarGenerationJobStatus(
	ctx context.Context, exec Executor, id uuid.UUID, status models.SarJobStatus, errMsg string,
) (bool, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return false, err
	}
	switch status {
	case models.SarJobStatusRunning, models.SarJobStatusDone, models.SarJobStatusFailed:
	default:
		return false, errors.Newf("unsupported sar job transition to %q", status)
	}
	sql, args, err := NewQueryBuilder().
		Update(dbmodels.TABLE_SAR_GENERATION_JOBS).
		Set("status", string(status)).
		Set("error", nullIfEmpty(errMsg)).
		Set("updated_at", squirrel.Expr("now()")).
		Where(squirrel.Eq{"id": id}).
		Where(squirrel.Eq{"status": []string{
			string(models.SarJobStatusPending), string(models.SarJobStatusRunning),
		}}).
		Suffix("RETURNING id").
		ToSql()
	if err != nil {
		return false, errors.Wrap(err, "cannot build sar job status query")
	}
	return execReturningRow(ctx, exec, sql, args)
}

// ResumeSarGenerationJob moves a job back to pending (error cleared, updated_at
// bumped), only from failed, or from pending/running with no update for more
// than staleAfter (database clock). It returns false when the job is not
// resumable, and models.ErrSarJobAlreadyActive when another job of the
// declaration is active.
func (repo *MarbleDbRepository) ResumeSarGenerationJob(
	ctx context.Context, exec Executor, id uuid.UUID, staleAfter time.Duration,
) (bool, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return false, err
	}
	sql, args, err := NewQueryBuilder().
		Update(dbmodels.TABLE_SAR_GENERATION_JOBS).
		Set("status", string(models.SarJobStatusPending)).
		Set("error", nil).
		Set("updated_at", squirrel.Expr("now()")).
		Where(squirrel.Eq{"id": id}).
		Where(squirrel.Or{
			squirrel.Eq{"status": string(models.SarJobStatusFailed)},
			squirrel.And{
				squirrel.Eq{"status": []string{
					string(models.SarJobStatusPending), string(models.SarJobStatusRunning),
				}},
				squirrel.Expr("updated_at < now() - make_interval(secs => ?)", staleAfter.Seconds()),
			},
		}).
		Suffix("RETURNING id").
		ToSql()
	if err != nil {
		return false, errors.Wrap(err, "cannot build sar job resume query")
	}
	ok, err := execReturningRow(ctx, exec, sql, args)
	if IsUniqueViolationErrorOf(err, sarActiveJobIndex) {
		return false, models.ErrSarJobAlreadyActive
	}
	return ok, err
}

// SetSarGenerationJobAttempt stores the River attempt number of the job.
func (repo *MarbleDbRepository) SetSarGenerationJobAttempt(
	ctx context.Context, exec Executor, id uuid.UUID, attempt int,
) error {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return err
	}
	query := NewQueryBuilder().
		Update(dbmodels.TABLE_SAR_GENERATION_JOBS).
		Set("attempt", attempt).
		Where(squirrel.Eq{"id": id})
	return ExecBuilder(ctx, exec, query)
}

// execReturningRow runs a statement ending with RETURNING and reports whether
// a row came back.
func execReturningRow(ctx context.Context, exec Executor, sql string, args []any) (bool, error) {
	rows, err := exec.Query(ctx, sql, args...)
	if err != nil {
		return false, errors.Wrap(err, "cannot update sar job")
	}
	defer rows.Close()
	found := rows.Next()
	if err := rows.Err(); err != nil {
		return false, err
	}
	return found, nil
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

// ListSarGenerationTaskSummaries is the light listing for progress views: no
// input, output or trace.
func (repo *MarbleDbRepository) ListSarGenerationTaskSummaries(
	ctx context.Context, exec Executor, jobId uuid.UUID,
) ([]models.SarGenerationTask, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, err
	}
	query := NewQueryBuilder().
		Select(dbmodels.SarGenerationTaskSummaryColumns...).
		From(dbmodels.TABLE_SAR_GENERATION_TASKS).
		Where(squirrel.Eq{"job_id": jobId}).
		OrderBy("created_at ASC", "id ASC")
	return SqlToListOfModels(ctx, exec, query, dbmodels.AdaptSarGenerationTaskSummary)
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
// started more than staleAfter ago (its worker is presumed dead). It returns the new
// attempt number and ok=true on success; ok=false when the task is not
// claimable (done, or running and fresh), so a task is never executed by two
// workers nor re-executed once done. The caller must pass the attempt number
// to Complete/FailSarGenerationTask: a superseded worker's result is refused.
//
// Staleness is evaluated in SQL against started_at with the database clock, so
// application clock skew is irrelevant.
func (repo *MarbleDbRepository) ClaimSarGenerationTask(
	ctx context.Context, exec Executor, id uuid.UUID, staleAfter time.Duration,
) (attempt int, ok bool, err error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return 0, false, err
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
				squirrel.Expr("started_at < now() - make_interval(secs => ?)", staleAfter.Seconds()),
			},
		}).
		Suffix("RETURNING attempts").
		ToSql()
	if err != nil {
		return 0, false, errors.Wrap(err, "cannot build sar task claim query")
	}
	rows, err := exec.Query(ctx, sql, args...)
	if err != nil {
		return 0, false, errors.Wrap(err, "cannot claim sar task")
	}
	defer rows.Close()
	if rows.Next() {
		if err := rows.Scan(&attempt); err != nil {
			return 0, false, errors.Wrap(err, "cannot read claimed sar task attempt")
		}
		ok = true
	}
	if err := rows.Err(); err != nil {
		return 0, false, errors.Wrap(err, "cannot claim sar task")
	}
	return attempt, ok, nil
}

// execGuardedTaskUpdate applies the update only if the task is still running
// at the given attempt; false means the claim was lost (task reclaimed or
// already finished) and the caller must discard its result.
func execGuardedTaskUpdate(
	ctx context.Context, exec Executor, query squirrel.UpdateBuilder, id uuid.UUID, attempt int,
) (bool, error) {
	sql, args, err := query.
		Where(squirrel.Eq{"id": id, "status": string(models.SarJobStatusRunning), "attempts": attempt}).
		Suffix("RETURNING id").
		ToSql()
	if err != nil {
		return false, errors.Wrap(err, "cannot build sar task update query")
	}
	rows, err := exec.Query(ctx, sql, args...)
	if err != nil {
		return false, errors.Wrap(err, "cannot update sar task")
	}
	defer rows.Close()
	updated := rows.Next()
	if err := rows.Err(); err != nil {
		return false, errors.Wrap(err, "cannot update sar task")
	}
	return updated, nil
}

// CompleteSarGenerationTask stores the output and trace and marks the task
// done, only if it is still running at the given attempt. It returns false
// when no row matched (superseded worker, or already finished).
func (repo *MarbleDbRepository) CompleteSarGenerationTask(
	ctx context.Context, exec Executor, id uuid.UUID, attempt int, output, trace []byte,
) (bool, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return false, err
	}
	query := NewQueryBuilder().
		Update(dbmodels.TABLE_SAR_GENERATION_TASKS).
		Set("status", string(models.SarJobStatusDone)).
		Set("output", rawOrNull(output)).
		Set("trace", rawOrNull(trace)).
		Set("finished_at", squirrel.Expr("now()"))
	return execGuardedTaskUpdate(ctx, exec, query, id, attempt)
}

// FailSarGenerationTask marks the task failed, with the same attempt guard as
// CompleteSarGenerationTask.
func (repo *MarbleDbRepository) FailSarGenerationTask(
	ctx context.Context, exec Executor, id uuid.UUID, attempt int, errMsg string, trace []byte,
) (bool, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return false, err
	}
	query := NewQueryBuilder().
		Update(dbmodels.TABLE_SAR_GENERATION_TASKS).
		Set("status", string(models.SarJobStatusFailed)).
		Set("error", nullIfEmpty(errMsg)).
		Set("trace", rawOrNull(trace)).
		Set("finished_at", squirrel.Expr("now()"))
	return execGuardedTaskUpdate(ctx, exec, query, id, attempt)
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
