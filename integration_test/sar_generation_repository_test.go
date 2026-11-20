package integration

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
	"github.com/riverqueue/river"
	"github.com/stretchr/testify/require"
)

// noopSarGenerationWorker only lets the harness River client validate and accept
// sar_generation jobs; it is never started on the org queues used in the tests.
type noopSarGenerationWorker struct {
	river.WorkerDefaults[models.SarGenerationArgs]
}

func (*noopSarGenerationWorker) Work(context.Context, *river.Job[models.SarGenerationArgs]) error {
	return nil
}

// mustSetJobStatus requires that a guarded job transition went through.
func mustSetJobStatus(t *testing.T, repo *repositories.MarbleDbRepository, exec repositories.Executor,
	id uuid.UUID, status models.SarJobStatus, errMsg string,
) {
	t.Helper()
	ok, err := repo.SetSarGenerationJobStatus(t.Context(), exec, id, status, errMsg)
	require.NoError(t, err)
	require.True(t, ok, "the job transition must be accepted")
}

// newSarGenTestJob builds a pending generation job for a declaration.
func newSarGenTestJob(declarationId uuid.UUID) models.SarGenerationJob {
	return models.SarGenerationJob{
		Id:            uuid.New(),
		DeclarationId: declarationId,
		Kind:          models.SarJobKindGenerate,
		Status:        models.SarJobStatusPending,
		Input:         []byte(`{}`),
	}
}

// newSarGenTestTask builds a pending task attached to a job.
func newSarGenTestTask(jobId uuid.UUID, targetPath string) models.SarGenerationTask {
	return models.SarGenerationTask{
		Id:         uuid.New(),
		JobId:      jobId,
		Kind:       "fill",
		TargetPath: targetPath,
		Input:      []byte(`{"path":"` + targetPath + `"}`),
		InputHash:  "hash-" + targetPath,
		DependsOn:  []uuid.UUID{},
		Status:     models.SarJobStatusPending,
	}
}

// sarGenTestSetup creates a declaration (with its tenant, org, inbox and case fixture)
// and a pending generation job for it, ready to receive tasks.
func sarGenTestSetup(t *testing.T) (*repositories.MarbleDbRepository, models.SarDeclaration, models.SarGenerationJob) {
	t.Helper()
	ctx := t.Context()
	organizationID, caseID := sarDeclarationTestFixture(t)
	repo := repositories.NewMarbleDbRepository(false, 0)
	exec := testUsecases.NewExecutorFactory().NewExecutor()

	declaration := newSarTestDeclaration(organizationID, caseID)
	require.NoError(t, repo.CreateSarDeclaration(ctx, exec, declaration))

	job := newSarGenTestJob(declaration.Id)
	require.NoError(t, repo.CreateSarGenerationJob(ctx, exec, job))
	return repo, declaration, job
}

func TestEnqueue_SecondActiveJobConflict(t *testing.T) {
	ctx := t.Context()
	repo, declaration, first := sarGenTestSetup(t)
	exec := testUsecases.NewExecutorFactory().NewExecutor()

	// The first job is pending: a second job for the same declaration is refused.
	second := newSarGenTestJob(declaration.Id)
	err := repo.CreateSarGenerationJob(ctx, exec, second)
	require.Error(t, err)
	require.True(t, errors.Is(err, models.ErrSarJobAlreadyActive))
	require.True(t, errors.Is(err, models.ConflictError))

	// Still refused while the first job is running.
	mustSetJobStatus(t, repo, exec, first.Id, models.SarJobStatusRunning, "")
	err = repo.CreateSarGenerationJob(ctx, exec, second)
	require.Error(t, err)
	require.True(t, errors.Is(err, models.ErrSarJobAlreadyActive))
	require.True(t, errors.Is(err, models.ConflictError))

	// Once the first job is done, a new job is accepted.
	mustSetJobStatus(t, repo, exec, first.Id, models.SarJobStatusDone, "")
	require.NoError(t, repo.CreateSarGenerationJob(ctx, exec, second))

	stored, err := repo.GetSarGenerationJob(ctx, exec, second.Id)
	require.NoError(t, err)
	require.Equal(t, declaration.Id, stored.DeclarationId)
	require.Equal(t, models.SarJobStatusPending, stored.Status)
}

func TestSarGenerationJob_FailedDoesNotBlockNewJob(t *testing.T) {
	ctx := t.Context()
	repo, declaration, first := sarGenTestSetup(t)
	exec := testUsecases.NewExecutorFactory().NewExecutor()

	mustSetJobStatus(t, repo, exec, first.Id, models.SarJobStatusFailed, "boom")
	stored, err := repo.GetSarGenerationJob(ctx, exec, first.Id)
	require.NoError(t, err)
	require.Equal(t, models.SarJobStatusFailed, stored.Status)
	require.Equal(t, "boom", stored.Error)

	require.NoError(t, repo.CreateSarGenerationJob(ctx, exec, newSarGenTestJob(declaration.Id)))
}

func TestClaimTask_StaleRunningIsClaimable(t *testing.T) {
	ctx := t.Context()
	repo, _, job := sarGenTestSetup(t)
	exec := testUsecases.NewExecutorFactory().NewExecutor()

	task := newSarGenTestTask(job.Id, "part.a")
	require.NoError(t, repo.CreateSarGenerationTasks(ctx, exec, []models.SarGenerationTask{task}))

	attempt1, claimed, err := repo.ClaimSarGenerationTask(ctx, exec, task.Id, time.Hour)
	require.NoError(t, err)
	require.True(t, claimed)
	require.Equal(t, 1, attempt1)

	// Simulate a worker that died long ago while the task was running.
	_, err = pgPool.Exec(ctx, "UPDATE sar_generation_tasks SET started_at = $1 WHERE id = $2",
		time.Now().Add(-2*time.Hour), task.Id)
	require.NoError(t, err)

	attempt2, claimed, err := repo.ClaimSarGenerationTask(ctx, exec, task.Id, time.Hour)
	require.NoError(t, err)
	require.True(t, claimed)
	require.Equal(t, 2, attempt2)

	tasks, err := repo.ListSarGenerationTasks(ctx, exec, job.Id)
	require.NoError(t, err)
	require.Len(t, tasks, 1)
	require.Equal(t, models.SarJobStatusRunning, tasks[0].Status)
	require.Equal(t, 2, tasks[0].Attempts)

	// The superseded first worker cannot complete or fail the task anymore.
	ok, err := repo.CompleteSarGenerationTask(ctx, exec, task.Id, attempt1, []byte(`{"old":true}`), nil)
	require.NoError(t, err)
	require.False(t, ok)
	ok, err = repo.FailSarGenerationTask(ctx, exec, task.Id, attempt1, "late", nil)
	require.NoError(t, err)
	require.False(t, ok)
	tasks, err = repo.ListSarGenerationTasks(ctx, exec, job.Id)
	require.NoError(t, err)
	require.Equal(t, models.SarJobStatusRunning, tasks[0].Status)
	require.Empty(t, tasks[0].Output)

	ok, err = repo.CompleteSarGenerationTask(ctx, exec, task.Id, attempt2, []byte(`{"new":true}`), nil)
	require.NoError(t, err)
	require.True(t, ok)
}

func TestClaimTask_FreshRunningIsNotClaimable(t *testing.T) {
	ctx := t.Context()
	repo, _, job := sarGenTestSetup(t)
	exec := testUsecases.NewExecutorFactory().NewExecutor()

	task := newSarGenTestTask(job.Id, "part.a")
	require.NoError(t, repo.CreateSarGenerationTasks(ctx, exec, []models.SarGenerationTask{task}))

	_, claimed, err := repo.ClaimSarGenerationTask(ctx, exec, task.Id, time.Hour)
	require.NoError(t, err)
	require.True(t, claimed)

	// The task just started: a claim whose stale threshold is one hour ago must be refused.
	_, claimed, err = repo.ClaimSarGenerationTask(ctx, exec, task.Id, time.Hour)
	require.NoError(t, err)
	require.False(t, claimed)

	tasks, err := repo.ListSarGenerationTasks(ctx, exec, job.Id)
	require.NoError(t, err)
	require.Len(t, tasks, 1)
	require.Equal(t, 1, tasks[0].Attempts)
}

func TestCompleteTask_StoresOutput(t *testing.T) {
	ctx := t.Context()
	repo, _, job := sarGenTestSetup(t)
	exec := testUsecases.NewExecutorFactory().NewExecutor()

	task := newSarGenTestTask(job.Id, "part.a")
	require.NoError(t, repo.CreateSarGenerationTasks(ctx, exec, []models.SarGenerationTask{task}))

	attempt, claimed, err := repo.ClaimSarGenerationTask(ctx, exec, task.Id, time.Hour)
	require.NoError(t, err)
	require.True(t, claimed)
	require.Equal(t, 1, attempt)

	output := []byte(`{"fills":[{"path":"part.a.nom","value":"Dupont"}]}`)
	trace := []byte(`[{"role":"assistant","content":"reasoning"}]`)
	ok, err := repo.CompleteSarGenerationTask(ctx, exec, task.Id, attempt, output, trace)
	require.NoError(t, err)
	require.True(t, ok)

	tasks, err := repo.ListSarGenerationTasks(ctx, exec, job.Id)
	require.NoError(t, err)
	require.Len(t, tasks, 1)
	require.Equal(t, models.SarJobStatusDone, tasks[0].Status)
	require.JSONEq(t, string(output), string(tasks[0].Output))
	require.NotNil(t, tasks[0].FinishedAt)
	require.Empty(t, tasks[0].Trace, "the list must not load the trace")

	gotTrace, err := repo.GetSarGenerationTaskTrace(ctx, exec, task.Id)
	require.NoError(t, err)
	require.JSONEq(t, string(trace), string(gotTrace))

	// Completing or failing again is refused and changes nothing.
	ok, err = repo.CompleteSarGenerationTask(ctx, exec, task.Id, attempt, []byte(`{"again":true}`), nil)
	require.NoError(t, err)
	require.False(t, ok)
	ok, err = repo.FailSarGenerationTask(ctx, exec, task.Id, attempt, "late", nil)
	require.NoError(t, err)
	require.False(t, ok)
	tasks, err = repo.ListSarGenerationTasks(ctx, exec, job.Id)
	require.NoError(t, err)
	require.Equal(t, models.SarJobStatusDone, tasks[0].Status)
	require.JSONEq(t, string(output), string(tasks[0].Output))

	// A done task is not claimable again, whatever the stale threshold.
	_, claimed, err = repo.ClaimSarGenerationTask(ctx, exec, task.Id, time.Hour)
	require.NoError(t, err)
	require.False(t, claimed)
}

func TestClaimTask_ConcurrentClaimExactlyOne(t *testing.T) {
	ctx := t.Context()
	repo, _, job := sarGenTestSetup(t)
	exec := testUsecases.NewExecutorFactory().NewExecutor()

	task := newSarGenTestTask(job.Id, "part.a")
	require.NoError(t, repo.CreateSarGenerationTasks(ctx, exec, []models.SarGenerationTask{task}))

	const workers = 8
	var wins atomic.Int32
	var wg sync.WaitGroup
	errs := make(chan error, workers)
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, ok, err := repo.ClaimSarGenerationTask(ctx, exec, task.Id, time.Hour)
			if err != nil {
				errs <- err
				return
			}
			if ok {
				wins.Add(1)
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}
	require.Equal(t, int32(1), wins.Load())
}

func TestEnqueueSarGenerationTask_InsertsRiverJob(t *testing.T) {
	ctx := t.Context()
	orgId, jobId, declarationId := uuid.New(), uuid.New(), uuid.New()
	taskQueue := repositories.NewTaskQueueRepository(riverClient)

	err := testUsecases.NewTransactionFactory().Transaction(ctx, func(tx repositories.Transaction) error {
		return taskQueue.EnqueueSarGenerationTask(ctx, tx, orgId, jobId, declarationId)
	})
	require.NoError(t, err)

	res, err := riverClient.JobList(ctx, river.NewJobListParams().Kinds("sar_generation").Queues(orgId.String()))
	require.NoError(t, err)
	require.Len(t, res.Jobs, 1)
	require.Equal(t, 5, res.Jobs[0].MaxAttempts)
	require.JSONEq(t,
		`{"job_id":"`+jobId.String()+`","declaration_id":"`+declarationId.String()+`"}`,
		string(res.Jobs[0].EncodedArgs))
}

func TestFailTask_StoresErrorAndClaimableAgain(t *testing.T) {
	ctx := t.Context()
	repo, _, job := sarGenTestSetup(t)
	exec := testUsecases.NewExecutorFactory().NewExecutor()

	task := newSarGenTestTask(job.Id, "part.a")
	require.NoError(t, repo.CreateSarGenerationTasks(ctx, exec, []models.SarGenerationTask{task}))

	attempt, claimed, err := repo.ClaimSarGenerationTask(ctx, exec, task.Id, time.Hour)
	require.NoError(t, err)
	require.True(t, claimed)

	trace := []byte(`[{"role":"assistant","content":"failed attempt"}]`)
	ok, err := repo.FailSarGenerationTask(ctx, exec, task.Id, attempt, "llm timeout", trace)
	require.NoError(t, err)
	require.True(t, ok)

	tasks, err := repo.ListSarGenerationTasks(ctx, exec, job.Id)
	require.NoError(t, err)
	require.Len(t, tasks, 1)
	require.Equal(t, models.SarJobStatusFailed, tasks[0].Status)
	require.Equal(t, "llm timeout", tasks[0].Error)

	gotTrace, err := repo.GetSarGenerationTaskTrace(ctx, exec, task.Id)
	require.NoError(t, err)
	require.JSONEq(t, string(trace), string(gotTrace))

	// A failed task can be claimed again, whatever the stale threshold.
	_, claimed, err = repo.ClaimSarGenerationTask(ctx, exec, task.Id, time.Hour)
	require.NoError(t, err)
	require.True(t, claimed)

	tasks, err = repo.ListSarGenerationTasks(ctx, exec, job.Id)
	require.NoError(t, err)
	require.Equal(t, models.SarJobStatusRunning, tasks[0].Status)
	require.Equal(t, 2, tasks[0].Attempts)
}

func TestSarQuestions_CreateListClose(t *testing.T) {
	ctx := t.Context()
	repo, declaration, _ := sarGenTestSetup(t)
	exec := testUsecases.NewExecutorFactory().NewExecutor()

	question := models.SarQuestion{
		Id:            uuid.New(),
		DeclarationId: declaration.Id,
		Kind:          "question",
		Paths:         []string{"organisme.nom"},
		Text:          "Quel est le nom exact de l'organisme ?",
		Options:       []byte(`[]`),
		Status:        "open",
	}
	finding := models.SarQuestion{
		Id:            uuid.New(),
		DeclarationId: declaration.Id,
		Kind:          "review_finding",
		Severity:      "important",
		Paths:         []string{"transactions.montant", "declarant.nom"},
		Text:          "Montant incohérent avec la transaction source.",
		Options:       []byte(`[]`),
		Status:        "open",
	}
	require.NoError(t, repo.CreateSarQuestions(ctx, exec, []models.SarQuestion{question, finding}))

	open, err := repo.ListSarQuestions(ctx, exec, declaration.Id, "open")
	require.NoError(t, err)
	require.Len(t, open, 2)

	gotFinding, err := repo.GetSarQuestion(ctx, exec, finding.Id)
	require.NoError(t, err)
	require.Equal(t, "review_finding", gotFinding.Kind)
	require.Equal(t, "important", gotFinding.Severity)
	require.Equal(t, []string{"transactions.montant", "declarant.nom"}, gotFinding.Paths)
	require.Equal(t, "open", gotFinding.Status)
	require.Nil(t, gotFinding.ClosedAt)

	require.NoError(t, repo.CloseSarQuestions(ctx, exec, []uuid.UUID{question.Id}, "resolved"))

	closed, err := repo.GetSarQuestion(ctx, exec, question.Id)
	require.NoError(t, err)
	require.Equal(t, "resolved", closed.Status)
	require.NotNil(t, closed.ClosedAt)

	open, err = repo.ListSarQuestions(ctx, exec, declaration.Id, "open")
	require.NoError(t, err)
	require.Len(t, open, 1)
	require.Equal(t, finding.Id, open[0].Id)
}

func TestListSarGenerationJobs_NewestFirstAndScopedToDeclaration(t *testing.T) {
	ctx := t.Context()
	repo, declaration, first := sarGenTestSetup(t)
	exec := testUsecases.NewExecutorFactory().NewExecutor()

	mustSetJobStatus(t, repo, exec, first.Id, models.SarJobStatusDone, "")
	second := newSarGenTestJob(declaration.Id)
	second.Kind = models.SarJobKindReview
	require.NoError(t, repo.CreateSarGenerationJob(ctx, exec, second))

	// A job of another declaration must not be listed.
	_, otherDeclaration, _ := sarGenTestSetup(t)

	jobs, err := repo.ListSarGenerationJobs(ctx, exec, declaration.Id)
	require.NoError(t, err)
	require.Len(t, jobs, 2)
	require.Equal(t, second.Id, jobs[0].Id, "newest first")
	require.Equal(t, first.Id, jobs[1].Id)

	other, err := repo.ListSarGenerationJobs(ctx, exec, otherDeclaration.Id)
	require.NoError(t, err)
	require.Len(t, other, 1)
	require.NotEqual(t, declaration.Id, other[0].DeclarationId)
}

func TestSarGenerationJob_GuardedTransitions(t *testing.T) {
	ctx := t.Context()
	repo, declaration, job := sarGenTestSetup(t)
	exec := testUsecases.NewExecutorFactory().NewExecutor()

	// pending -> running is accepted, and so is running -> running (heartbeat).
	mustSetJobStatus(t, repo, exec, job.Id, models.SarJobStatusRunning, "")
	mustSetJobStatus(t, repo, exec, job.Id, models.SarJobStatusRunning, "")

	// Resume is refused while the job is fresh.
	ok, err := repo.ResumeSarGenerationJob(ctx, exec, job.Id, time.Hour)
	require.NoError(t, err)
	require.False(t, ok)

	// done is final: no transition leaves it.
	mustSetJobStatus(t, repo, exec, job.Id, models.SarJobStatusDone, "")
	for _, st := range []models.SarJobStatus{models.SarJobStatusRunning, models.SarJobStatusFailed, models.SarJobStatusDone} {
		ok, err := repo.SetSarGenerationJobStatus(ctx, exec, job.Id, st, "late")
		require.NoError(t, err)
		require.False(t, ok, "done -> %s must be refused", st)
	}
	ok, err = repo.ResumeSarGenerationJob(ctx, exec, job.Id, 0)
	require.NoError(t, err)
	require.False(t, ok, "a done job is never resumed")

	// pending is only reachable through ResumeSarGenerationJob.
	_, err = repo.SetSarGenerationJobStatus(ctx, exec, job.Id, models.SarJobStatusPending, "")
	require.Error(t, err)

	// failed -> pending through Resume; the error is cleared.
	second := newSarGenTestJob(declaration.Id)
	require.NoError(t, repo.CreateSarGenerationJob(ctx, exec, second))
	mustSetJobStatus(t, repo, exec, second.Id, models.SarJobStatusFailed, "boom")
	ok, err = repo.ResumeSarGenerationJob(ctx, exec, second.Id, time.Hour)
	require.NoError(t, err)
	require.True(t, ok)
	stored, err := repo.GetSarGenerationJob(ctx, exec, second.Id)
	require.NoError(t, err)
	require.Equal(t, models.SarJobStatusPending, stored.Status)
	require.Empty(t, stored.Error)
}

func TestSarGenerationJob_StaleResumeUsesDatabaseClock(t *testing.T) {
	ctx := t.Context()
	repo, _, job := sarGenTestSetup(t)
	exec := testUsecases.NewExecutorFactory().NewExecutor()

	// A pending job nobody picked up, 2 hours old.
	_, err := pgPool.Exec(ctx, "UPDATE sar_generation_jobs SET updated_at = now() - interval '2 hours' WHERE id = $1", job.Id)
	require.NoError(t, err)
	ok, err := repo.ResumeSarGenerationJob(ctx, exec, job.Id, 3*time.Hour)
	require.NoError(t, err)
	require.False(t, ok, "not stale yet")
	ok, err = repo.ResumeSarGenerationJob(ctx, exec, job.Id, time.Hour)
	require.NoError(t, err)
	require.True(t, ok, "stale pending job is resumable")
}

func TestSarGenerationJob_AttemptAndLightTaskListing(t *testing.T) {
	ctx := t.Context()
	repo, _, job := sarGenTestSetup(t)
	exec := testUsecases.NewExecutorFactory().NewExecutor()

	require.NoError(t, repo.SetSarGenerationJobAttempt(ctx, exec, job.Id, 3))
	stored, err := repo.GetSarGenerationJob(ctx, exec, job.Id)
	require.NoError(t, err)
	require.Equal(t, 3, stored.Attempt)

	task := newSarGenTestTask(job.Id, "part.a")
	require.NoError(t, repo.CreateSarGenerationTasks(ctx, exec, []models.SarGenerationTask{task}))
	attempt, claimed, err := repo.ClaimSarGenerationTask(ctx, exec, task.Id, time.Hour)
	require.NoError(t, err)
	require.True(t, claimed)
	ok, err := repo.CompleteSarGenerationTask(ctx, exec, task.Id, attempt, []byte(`{"applied":[]}`), []byte(`[1]`))
	require.NoError(t, err)
	require.True(t, ok)

	light, err := repo.ListSarGenerationTaskSummaries(ctx, exec, job.Id)
	require.NoError(t, err)
	require.Len(t, light, 1)
	require.Equal(t, task.Id, light[0].Id)
	require.Equal(t, models.SarJobStatusDone, light[0].Status)
	require.Equal(t, "part.a", light[0].TargetPath)
	require.Empty(t, light[0].Input)
	require.Empty(t, light[0].Output)
	require.Empty(t, light[0].Trace)
	require.NotNil(t, light[0].FinishedAt)
}

func TestSarGenerationJob_RateLimitStreak(t *testing.T) {
	ctx := t.Context()
	repo, _, job := sarGenTestSetup(t)
	exec := testUsecases.NewExecutorFactory().NewExecutor()

	elapsed, err := repo.MarkSarGenerationJobRateLimited(ctx, exec, job.Id)
	require.NoError(t, err)
	require.Less(t, elapsed, time.Minute, "the first rate limit starts the streak")
	stored, err := repo.GetSarGenerationJob(ctx, exec, job.Id)
	require.NoError(t, err)
	require.NotNil(t, stored.RateLimitedSince)

	// The streak keeps its start (database clock).
	_, err = pgPool.Exec(ctx, "UPDATE sar_generation_jobs SET rate_limited_since = now() - interval '3 hours' WHERE id = $1", job.Id)
	require.NoError(t, err)
	elapsed, err = repo.MarkSarGenerationJobRateLimited(ctx, exec, job.Id)
	require.NoError(t, err)
	require.Greater(t, elapsed, 2*time.Hour+50*time.Minute)

	require.NoError(t, repo.ClearSarGenerationJobRateLimited(ctx, exec, job.Id))
	stored, err = repo.GetSarGenerationJob(ctx, exec, job.Id)
	require.NoError(t, err)
	require.Nil(t, stored.RateLimitedSince)

	// A resume also starts a fresh streak.
	_, err = repo.MarkSarGenerationJobRateLimited(ctx, exec, job.Id)
	require.NoError(t, err)
	mustSetJobStatus(t, repo, exec, job.Id, models.SarJobStatusFailed, "x")
	ok, err := repo.ResumeSarGenerationJob(ctx, exec, job.Id, time.Hour)
	require.NoError(t, err)
	require.True(t, ok)
	stored, err = repo.GetSarGenerationJob(ctx, exec, job.Id)
	require.NoError(t, err)
	require.Nil(t, stored.RateLimitedSince)
}
