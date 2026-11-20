package repositories

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
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
		Set("rate_limited_since", nil).
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

// FailStaleSarGenerationJobs moves the pending/running jobs of a declaration
// with no update for more than staleAfter (database clock) to failed, with the
// given error message. It returns the number of jobs failed.
func (repo *MarbleDbRepository) FailStaleSarGenerationJobs(
	ctx context.Context, exec Executor, declarationId uuid.UUID, staleAfter time.Duration, errMsg string,
) (int, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return 0, err
	}
	sql, args, err := NewQueryBuilder().
		Update(dbmodels.TABLE_SAR_GENERATION_JOBS).
		Set("status", string(models.SarJobStatusFailed)).
		Set("error", nullIfEmpty(errMsg)).
		Set("updated_at", squirrel.Expr("now()")).
		Where(squirrel.Eq{"declaration_id": declarationId}).
		Where(squirrel.Eq{"status": []string{
			string(models.SarJobStatusPending), string(models.SarJobStatusRunning),
		}}).
		Where(squirrel.Expr("updated_at < now() - make_interval(secs => ?)", staleAfter.Seconds())).
		Suffix("RETURNING id").
		ToSql()
	if err != nil {
		return 0, errors.Wrap(err, "cannot build sar stale jobs query")
	}
	rows, err := exec.Query(ctx, sql, args...)
	if err != nil {
		return 0, errors.Wrap(err, "cannot fail stale sar jobs")
	}
	defer rows.Close()
	n := 0
	for rows.Next() {
		n++
	}
	return n, rows.Err()
}

// DeleteSarGenerationTasks deletes every task of a job, done ones included: a
// failed job whose tasks wrote nothing to the document is resumed from a fresh
// plan once its inputs (the selection) changed.
func (repo *MarbleDbRepository) DeleteSarGenerationTasks(
	ctx context.Context, exec Executor, jobId uuid.UUID,
) error {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return err
	}
	query := NewQueryBuilder().
		Delete(dbmodels.TABLE_SAR_GENERATION_TASKS).
		Where(squirrel.Eq{"job_id": jobId})
	return ExecBuilder(ctx, exec, query)
}

// DeleteSarGenerationUnfinishedTasks deletes, among the given tasks of the job,
// those that are not done: a task is only deleted when it belongs to the job and
// has not completed, so a done task (whose output may already be applied) is
// never touched. A worker still running a deleted task has its result refused
// by the attempt guard.
func (repo *MarbleDbRepository) DeleteSarGenerationUnfinishedTasks(
	ctx context.Context, exec Executor, jobId uuid.UUID, taskIds []uuid.UUID,
) error {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return err
	}
	if len(taskIds) == 0 {
		return nil
	}
	query := NewQueryBuilder().
		Delete(dbmodels.TABLE_SAR_GENERATION_TASKS).
		Where(squirrel.Eq{"job_id": jobId, "id": taskIds}).
		Where(squirrel.NotEq{"status": string(models.SarJobStatusDone)})
	return ExecBuilder(ctx, exec, query)
}

// SarGenerationJobWroteToDocument tells whether a done task of the job changed
// the declaration document. Tasks record it in their output ("wrote"); an
// output without the key (older rows) counts as a write, the safe side for the
// selection freeze.
func (repo *MarbleDbRepository) SarGenerationJobWroteToDocument(
	ctx context.Context, exec Executor, jobId uuid.UUID,
) (bool, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return false, err
	}
	sql, args, err := NewQueryBuilder().
		Select("1").
		From(dbmodels.TABLE_SAR_GENERATION_TASKS).
		Where(squirrel.Eq{"job_id": jobId, "status": string(models.SarJobStatusDone)}).
		Where("coalesce((output->>'wrote')::boolean, true)").
		Limit(1).
		ToSql()
	if err != nil {
		return false, errors.Wrap(err, "cannot build sar job writes query")
	}
	rows, err := exec.Query(ctx, sql, args...)
	if err != nil {
		return false, errors.Wrap(err, "cannot read sar job writes")
	}
	defer rows.Close()
	found := rows.Next()
	return found, rows.Err()
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

// MarkSarGenerationJobRateLimited records a provider rate limit: the first one
// of a streak sets rate_limited_since (database clock). It returns how long the
// streak has lasted.
func (repo *MarbleDbRepository) MarkSarGenerationJobRateLimited(
	ctx context.Context, exec Executor, id uuid.UUID,
) (time.Duration, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return 0, err
	}
	rows, err := exec.Query(ctx, `UPDATE `+dbmodels.TABLE_SAR_GENERATION_JOBS+`
		SET rate_limited_since = COALESCE(rate_limited_since, now()), updated_at = now()
		WHERE id = $1
		RETURNING extract(epoch FROM (now() - rate_limited_since))::float8`, id)
	if err != nil {
		return 0, errors.Wrap(err, "cannot mark sar job rate limited")
	}
	defer rows.Close()
	if !rows.Next() {
		return 0, rows.Err()
	}
	var secs float64
	if err := rows.Scan(&secs); err != nil {
		return 0, errors.Wrap(err, "cannot read rate limit duration")
	}
	return time.Duration(secs * float64(time.Second)), nil
}

// ClearSarGenerationJobRateLimited ends a rate-limit streak (a task succeeded).
func (repo *MarbleDbRepository) ClearSarGenerationJobRateLimited(
	ctx context.Context, exec Executor, id uuid.UUID,
) error {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return err
	}
	query := NewQueryBuilder().
		Update(dbmodels.TABLE_SAR_GENERATION_JOBS).
		Set("rate_limited_since", nil).
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

// CreateSarGenerationTasks inserts the tasks of pending or running jobs; a task
// whose (job, input hash) already exists is skipped (concurrent planners share
// the same rows), and so is a task of a job that is already finished (the job
// status is checked in the same statement, so a late planner cannot attach
// tasks to a done or failed job). The caller must re-read the tasks of the job.
func (repo *MarbleDbRepository) CreateSarGenerationTasks(
	ctx context.Context, exec Executor, tasks []models.SarGenerationTask,
) error {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return err
	}
	if len(tasks) == 0 {
		return nil
	}
	const columns = "id, job_id, kind, target_path, input, input_hash, depends_on, status"
	var (
		sb   strings.Builder
		args []any
	)
	sb.WriteString("INSERT INTO " + dbmodels.TABLE_SAR_GENERATION_TASKS + " (" + columns + ") ")
	sb.WriteString("SELECT v.id, v.job_id, v.kind, v.target_path, v.input, v.input_hash, v.depends_on, v.status FROM (VALUES ")
	for i, t := range tasks {
		dependsOn := t.DependsOn
		if dependsOn == nil {
			dependsOn = []uuid.UUID{}
		}
		if i > 0 {
			sb.WriteString(", ")
		}
		n := len(args)
		fmt.Fprintf(&sb, "($%d::uuid, $%d::uuid, $%d::text, $%d::text, $%d::jsonb, $%d::text, $%d::uuid[], $%d::text)",
			n+1, n+2, n+3, n+4, n+5, n+6, n+7, n+8)
		args = append(args, t.Id, t.JobId, t.Kind, nullIfEmpty(t.TargetPath),
			json.RawMessage(t.Input), t.InputHash, dependsOn, string(t.Status))
	}
	sb.WriteString(") AS v(" + columns + ") WHERE EXISTS (SELECT 1 FROM " + dbmodels.TABLE_SAR_GENERATION_JOBS +
		" j WHERE j.id = v.job_id AND j.status IN ('" + string(models.SarJobStatusPending) + "', '" +
		string(models.SarJobStatusRunning) + "')) ON CONFLICT (job_id, input_hash) DO NOTHING")
	if _, err := exec.Exec(ctx, sb.String(), args...); err != nil {
		return errors.Wrap(err, "cannot create sar generation tasks")
	}
	return nil
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

// CloseSarQuestions sets the status (resolved, dismissed or replaced) and closed_at.
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
