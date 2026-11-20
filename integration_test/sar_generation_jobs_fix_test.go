package integration

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/usecases/sargen/core"
	"github.com/checkmarble/marble-backend/usecases/sargen/engine"
)

// Fix wave PR 4 (J2, J4), against the real database.

// jobsBarrierFormat makes the first Plan call of each of two concurrent
// RunJob wait until both have listed the tasks of the job: two River jobs
// started on the same job at the same time.
type jobsBarrierFormat struct {
	*engFormat
	mu      sync.Mutex
	arrived int
	release chan struct{}
}

func (f *jobsBarrierFormat) Plan(st core.PlanState) []core.TaskSpec {
	f.mu.Lock()
	f.arrived++
	n := f.arrived
	f.mu.Unlock()
	if n == 2 {
		close(f.release)
	}
	if n <= 2 {
		select {
		case <-f.release:
		case <-time.After(5 * time.Second):
		}
	}
	return f.engFormat.Plan(st)
}

// ---- J2: double execution of a job ----

// PR 4 I1 (verification-jobs item 2): two RunJob on the same job must create
// one task per input hash and apply its output once. Two rows for one hash
// means the work is run and written twice.
func TestSarEngine_TwoRunJobsSameJobCreateOneTask(t *testing.T) {
	h := newEngHarness(t, time.Hour)
	h.format.waves = [][]core.TaskSpec{{engSpec("create", "personnes.pp")}}
	h.runner.steps["create"] = func(int) (core.TaskOutput, error) {
		return core.TaskOutput{Fills: []core.Fill{engFill("personnes.pp[+n].etatCivil.nomNaissance", "MARTIN")}}, nil
	}
	bf := &jobsBarrierFormat{engFormat: h.format, release: make(chan struct{})}
	h.orch = engine.NewOrchestrator(engine.Deps{
		ExecutorFactory:    testUsecases.NewExecutorFactory(),
		TransactionFactory: testUsecases.NewTransactionFactory(),
		Declarations:       h.repo, Selection: h.repo, Generation: h.repo,
		DataModels: engDataModels{}, Runner: h.runner, Prompts: engPrompts{},
		FormatGetter: func(core.FormatKey) (core.Format, bool) { return bf, true },
		Parallelism:  1, StaleAfter: time.Hour,
	})

	var wg sync.WaitGroup
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = h.orch.RunJob(context.Background(), h.job.Id)
		}()
	}
	wg.Wait()

	tasks, err := h.repo.ListSarGenerationTasks(t.Context(), h.exec, h.job.Id)
	require.NoError(t, err)
	created := 0
	for _, tk := range tasks {
		if tk.Kind == "create" {
			created++
		}
	}
	require.Equal(t, 1, created, "one task per input hash")
	require.Equal(t, 1, h.runner.count("create"), "the task runs once")

	_, doc := h.declaration(t)
	require.Len(t, doc.Items["personnes.pp"], 2, "the created person is applied once")
}

// PR 4 I1: an output that arrives after the job was failed (by another
// attempt or an abandon) must not be applied: the declaration stays as it
// was and the job stays failed.
func TestSarEngine_OutputNotAppliedAfterJobFailed(t *testing.T) {
	h := newEngHarness(t, time.Hour)
	h.format.waves = [][]core.TaskSpec{{engSpec("a", engPP1)}}
	h.runner.steps["a"] = func(int) (core.TaskOutput, error) {
		// Meanwhile the job is failed by someone else.
		ok, err := h.repo.SetSarGenerationJobStatus(context.Background(), h.exec, h.job.Id, models.SarJobStatusFailed, "x")
		require.NoError(t, err)
		require.True(t, ok)
		return core.TaskOutput{Fills: []core.Fill{engFill(engPP1Name, "DUPONT")}}, nil
	}

	_ = h.orch.RunJob(t.Context(), h.job.Id)

	d, doc := h.declaration(t)
	require.Equal(t, 1, d.Version, "nothing written to the document")
	_, applied := doc.Values[engPP1Name]
	require.False(t, applied, "the output of a failed job is not applied")
	require.Equal(t, models.SarJobStatusFailed, h.jobStatus(t).Status)
}

// ---- J4: schema of the jobs table ----

// PR 5 migration (verification-foundations item 5): the jobs table carries
// rate_limited_since, nullable, timestamp with time zone, as on the agent
// branch.
func TestSarGenerationJobs_RateLimitedSinceColumn(t *testing.T) {
	rows, err := pgPool.Query(t.Context(),
		`SELECT data_type, is_nullable FROM information_schema.columns
		 WHERE table_schema = current_schema() AND table_name = 'sar_generation_jobs'
		   AND column_name = 'rate_limited_since'`)
	require.NoError(t, err)
	defer rows.Close()
	var cols [][2]string
	for rows.Next() {
		var dataType, nullable string
		require.NoError(t, rows.Scan(&dataType, &nullable))
		cols = append(cols, [2]string{dataType, nullable})
	}
	require.NoError(t, rows.Err())
	require.Len(t, cols, 1, "column sar_generation_jobs.rate_limited_since must exist")
	require.Equal(t, "timestamp with time zone", cols[0][0])
	require.Equal(t, "YES", cols[0][1], "nullable")
}

// ---- Round 1: FailStaleSarGenerationJobs against the real database ----

func TestSarGenerationJobs_FailStale(t *testing.T) {
	h := newEngHarness(t, time.Hour)
	repo, declId := h.repo, h.job.DeclarationId
	ctx := t.Context()
	age := func(id any, d string) {
		_, err := pgPool.Exec(ctx, `UPDATE sar_generation_jobs SET updated_at = now() - $2::interval WHERE id = $1`, id, d)
		require.NoError(t, err)
	}
	status := func() models.SarGenerationJob {
		j, err := repo.GetSarGenerationJob(ctx, h.exec, h.job.Id)
		require.NoError(t, err)
		return j
	}

	// Fresh active job: untouched.
	n, err := repo.FailStaleSarGenerationJobs(ctx, h.exec, declId, time.Hour, "arrêtée")
	require.NoError(t, err)
	require.Zero(t, n)
	require.NotEqual(t, models.SarJobStatusFailed, status().Status)

	// Old active job: failed with the message.
	age(h.job.Id, "3 hours")
	n, err = repo.FailStaleSarGenerationJobs(ctx, h.exec, declId, time.Hour, "arrêtée")
	require.NoError(t, err)
	require.Equal(t, 1, n)
	j := status()
	require.Equal(t, models.SarJobStatusFailed, j.Status)
	require.Equal(t, "arrêtée", j.Error)

	// Already failed job: untouched (error kept).
	age(h.job.Id, "5 hours")
	n, err = repo.FailStaleSarGenerationJobs(ctx, h.exec, declId, time.Hour, "autre")
	require.NoError(t, err)
	require.Zero(t, n)
	require.Equal(t, "arrêtée", status().Error)
}

// DeleteSarGenerationTasks removes every task of the job, done ones included;
// other jobs are untouched. SarGenerationJobWroteToDocument only counts done
// tasks whose output did not record wrote=false.
func TestSarGenerationTasks_DeleteAllAndWroteFlag(t *testing.T) {
	ctx := t.Context()
	repo, _, job := sarGenTestSetup(t)
	exec := testUsecases.NewExecutorFactory().NewExecutor()

	pending := newSarGenTestTask(job.Id, "part.pending")
	running := newSarGenTestTask(job.Id, "part.running")
	failed := newSarGenTestTask(job.Id, "part.failed")
	done := newSarGenTestTask(job.Id, "part.done")
	require.NoError(t, repo.CreateSarGenerationTasks(ctx, exec,
		[]models.SarGenerationTask{pending, running, failed, done}))

	claim := func(id uuid.UUID) int {
		attempt, claimed, err := repo.ClaimSarGenerationTask(ctx, exec, id, time.Hour)
		require.NoError(t, err)
		require.True(t, claimed)
		return attempt
	}
	claim(running.Id)
	attempt := claim(failed.Id)
	ok, err := repo.FailSarGenerationTask(ctx, exec, failed.Id, attempt, "boom", nil)
	require.NoError(t, err)
	require.True(t, ok)
	attempt = claim(done.Id)
	ok, err = repo.CompleteSarGenerationTask(ctx, exec, done.Id, attempt, []byte(`{}`), nil)
	require.NoError(t, err)
	require.True(t, ok)

	wrote, err := repo.SarGenerationJobWroteToDocument(ctx, exec, job.Id)
	require.NoError(t, err)
	require.True(t, wrote, "an output without the wrote key counts as a write")

	// A task of another job must survive.
	_, _, otherJob := sarGenTestSetup(t)
	otherTask := newSarGenTestTask(otherJob.Id, "part.other")
	require.NoError(t, repo.CreateSarGenerationTasks(ctx, exec, []models.SarGenerationTask{otherTask}))

	require.NoError(t, repo.DeleteSarGenerationTasks(ctx, exec, job.Id))

	tasks, err := repo.ListSarGenerationTasks(ctx, exec, job.Id)
	require.NoError(t, err)
	require.Empty(t, tasks)

	others, err := repo.ListSarGenerationTasks(ctx, exec, otherJob.Id)
	require.NoError(t, err)
	require.Len(t, others, 1)

	// A done task recording wrote=false does not count.
	readOnly := newSarGenTestTask(otherJob.Id, "part.readonly")
	require.NoError(t, repo.CreateSarGenerationTasks(ctx, exec, []models.SarGenerationTask{readOnly}))
	attempt, claimed, err := repo.ClaimSarGenerationTask(ctx, exec, readOnly.Id, time.Hour)
	require.NoError(t, err)
	require.True(t, claimed)
	ok, err = repo.CompleteSarGenerationTask(ctx, exec, readOnly.Id, attempt, []byte(`{"wrote":false}`), nil)
	require.NoError(t, err)
	require.True(t, ok)
	wrote, err = repo.SarGenerationJobWroteToDocument(ctx, exec, otherJob.Id)
	require.NoError(t, err)
	require.False(t, wrote)
}

// Tasks are only inserted into a pending or running job: a finished job gets
// none, and the rows of an active job are unaffected.
func TestSarGenerationTasks_CreateOnlyForActiveJob(t *testing.T) {
	ctx := t.Context()
	repo, _, job := sarGenTestSetup(t)
	exec := testUsecases.NewExecutorFactory().NewExecutor()

	first := newSarGenTestTask(job.Id, "part.a")
	require.NoError(t, repo.CreateSarGenerationTasks(ctx, exec, []models.SarGenerationTask{first}))

	mustSetJobStatus(t, repo, exec, job.Id, models.SarJobStatusRunning, "")
	second := newSarGenTestTask(job.Id, "part.b")
	require.NoError(t, repo.CreateSarGenerationTasks(ctx, exec, []models.SarGenerationTask{second}))
	tasks, err := repo.ListSarGenerationTasks(ctx, exec, job.Id)
	require.NoError(t, err)
	require.Len(t, tasks, 2, "a running job accepts new tasks")

	for _, status := range []models.SarJobStatus{models.SarJobStatusFailed, models.SarJobStatusDone} {
		_, _, finished := sarGenTestSetup(t)
		mustSetJobStatus(t, repo, exec, finished.Id, status, "")
		late := newSarGenTestTask(finished.Id, "part.late")
		require.NoError(t, repo.CreateSarGenerationTasks(ctx, exec, []models.SarGenerationTask{late}))
		got, err := repo.ListSarGenerationTasks(ctx, exec, finished.Id)
		require.NoError(t, err)
		require.Empty(t, got, "a %s job must not receive tasks", status)
	}
}

// Final review M1: an output discarded because the job is no longer running
// releases the task (failed, claimable again) and stops that run, so a
// resumed job is not blocked until the task goes stale.
func TestSarEngine_DiscardedOutputReleasesTaskAndResumeProgresses(t *testing.T) {
	h := newEngHarness(t, time.Hour)
	h.format.waves = [][]core.TaskSpec{{engSpec("a", engPP1)}}
	first := true
	h.runner.steps["a"] = func(int) (core.TaskOutput, error) {
		if first {
			first = false
			ok, err := h.repo.SetSarGenerationJobStatus(context.Background(), h.exec, h.job.Id, models.SarJobStatusFailed, "stale")
			require.NoError(t, err)
			require.True(t, ok)
		}
		return core.TaskOutput{Fills: []core.Fill{engFill(engPP1Name, "DUPONT")}}, nil
	}

	err := h.orch.RunJob(t.Context(), h.job.Id)
	require.ErrorIs(t, err, engine.ErrJobNotRunning, "the zombie run stops")
	require.Equal(t, models.SarJobStatusFailed, h.task(t, "a").Status, "the task is released")
	require.Equal(t, 1, h.runner.count("a"))

	ok, err := h.repo.ResumeSarGenerationJob(t.Context(), h.exec, h.job.Id, time.Hour)
	require.NoError(t, err)
	require.True(t, ok)
	require.NoError(t, h.orch.RunJob(t.Context(), h.job.Id), "the resumed job is not blocked by the discarded task")
	require.Equal(t, models.SarJobStatusDone, h.jobStatus(t).Status)
	_, doc := h.declaration(t)
	require.Equal(t, "DUPONT", doc.Values[engPP1Name].Value)
}

// DeleteSarGenerationUnfinishedTasks removes only the given not-done tasks of
// the job: a done task, a task not listed and a task of another job survive.
func TestSarGenerationTasks_DeleteUnfinishedOnly(t *testing.T) {
	ctx := t.Context()
	repo, _, job := sarGenTestSetup(t)
	exec := testUsecases.NewExecutorFactory().NewExecutor()

	stale := newSarGenTestTask(job.Id, "part.stale")
	kept := newSarGenTestTask(job.Id, "part.kept")
	done := newSarGenTestTask(job.Id, "part.done")
	require.NoError(t, repo.CreateSarGenerationTasks(ctx, exec, []models.SarGenerationTask{stale, kept, done}))
	attempt, claimed, err := repo.ClaimSarGenerationTask(ctx, exec, done.Id, time.Hour)
	require.NoError(t, err)
	require.True(t, claimed)
	ok, err := repo.CompleteSarGenerationTask(ctx, exec, done.Id, attempt, []byte(`{}`), nil)
	require.NoError(t, err)
	require.True(t, ok)

	_, _, otherJob := sarGenTestSetup(t)
	other := newSarGenTestTask(otherJob.Id, "part.other")
	require.NoError(t, repo.CreateSarGenerationTasks(ctx, exec, []models.SarGenerationTask{other}))

	require.NoError(t, repo.DeleteSarGenerationUnfinishedTasks(ctx, exec, job.Id,
		[]uuid.UUID{stale.Id, done.Id, other.Id}))

	tasks, err := repo.ListSarGenerationTasks(ctx, exec, job.Id)
	require.NoError(t, err)
	require.Len(t, tasks, 2)
	ids := []uuid.UUID{tasks[0].Id, tasks[1].Id}
	require.ElementsMatch(t, []uuid.UUID{kept.Id, done.Id}, ids)
	others, err := repo.ListSarGenerationTasks(ctx, exec, otherJob.Id)
	require.NoError(t, err)
	require.Len(t, others, 1)
}
