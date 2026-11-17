package engine

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
	"github.com/riverqueue/river"
	"github.com/stretchr/testify/require"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/usecases/sargen/core"
)

// C1: a job starts pending; RunJob makes it running (and bumps updated_at).
func TestRunJob_JobBecomesRunning(t *testing.T) {
	o, f := newTestOrchestrator(t)
	seedDocument(t, f, withTwoPersons)
	require.Equal(t, models.SarJobStatusPending, jobOf(t, f).Status)
	f.Advance(5 * 60e9)

	var during models.SarJobStatus
	f.Format.Waves = [][]core.TaskSpec{{Spec("a", tgtPP1)}}
	f.Runner.On("a", tgtPP1, Step{Before: func() { during = jobOf(t, f).Status }})

	require.NoError(t, o.RunJob(context.Background(), f.JobId))

	require.Equal(t, models.SarJobStatusRunning, during, "the job is running while its tasks run")
	require.Equal(t, []models.SarJobStatus{models.SarJobStatusRunning, models.SarJobStatusDone}, f.Repo.StatusHistory)
}

// A finished job is left alone (guarded transition).
func TestRunJob_FinishedJobIsNotRestarted(t *testing.T) {
	o, f := newTestOrchestrator(t)
	f.Repo.jobs[f.JobId].Status = models.SarJobStatusFailed
	f.Format.Waves = [][]core.TaskSpec{{Spec("a", tgtPP1)}}

	require.NoError(t, o.RunJob(context.Background(), f.JobId))
	require.Equal(t, models.SarJobStatusFailed, jobOf(t, f).Status)
	require.Empty(t, f.Runner.Calls)
}

// I1: question paths and option fills stay within the task's part.
func TestRunJob_QuestionOutsideTargetIsNotPersisted(t *testing.T) {
	ctx := context.Background()
	o, f := newTestOrchestrator(t)
	seedDocument(t, f, withTwoPersons)

	const unresolved = "personnes.pp[+z].etatCivil.nomNaissance"
	f.Format.Waves = [][]core.TaskSpec{{Spec("a", tgtPP1)}}
	f.Runner.On("a", tgtPP1, Step{Out: core.TaskOutput{Questions: []core.QuestionDraft{
		{
			// Mixed: one path inside, one in another person, one never created.
			Paths: []core.Path{pathImplPP1, pathNomPP2, unresolved},
			Text:  "Implication ?",
			Options: []core.QuestionOption{{Label: "x", Fills: []core.Fill{
				{Path: pathImplPP1, Value: "v"},
				{Path: pathNomPP2, Value: "INTRUS"}, // outside the kept paths
			}}},
		},
		{
			// Every path outside: persisted path-less, its option cannot write.
			Paths:   []core.Path{pathNomPP2},
			Text:    "Nom de l'autre personne ?",
			Options: []core.QuestionOption{{Label: "y", Fills: []core.Fill{{Path: pathNomPP2, Value: "Z"}}}},
		},
	}}})

	require.NoError(t, o.RunJob(ctx, f.JobId))

	byText := map[string]models.SarQuestion{}
	for _, q := range questionsOf(t, f, "") {
		byText[q.Text] = q
	}
	require.Len(t, byText, 2)

	mixed := byText["Implication ?"]
	require.Equal(t, []string{pathImplPP1}, mixed.Paths)
	var opts []core.QuestionOption
	require.NoError(t, json.Unmarshal(mixed.Options, &opts))
	require.Len(t, opts[0].Fills, 1)
	require.Equal(t, core.Path(pathImplPP1), opts[0].Fills[0].Path)

	other := byText["Nom de l'autre personne ?"]
	require.Empty(t, other.Paths)
	require.NoError(t, json.Unmarshal(other.Options, &opts))
	require.Empty(t, opts[0].Fills)

	var res taskResult
	require.NoError(t, json.Unmarshal(f.Repo.Task(t, "a", tgtPP1).Output, &res))
	var rejected []string
	for _, r := range res.Rejected {
		rejected = append(rejected, string(r.Path)+"|"+r.Code)
	}
	require.ElementsMatch(t, []string{
		pathNomPP2 + "|" + core.CodeOutOfScope, // question path (mixed)
		unresolved + "|" + CodeUnresolvedPath,  // question path (mixed)
		pathNomPP2 + "|" + core.CodeOutOfScope, // option fill (mixed)
		pathNomPP2 + "|" + core.CodeOutOfScope, // question path (other)
		pathNomPP2 + "|" + core.CodeOutOfScope, // option fill (other)
	}, rejected)
}

// I2: a planned compute task runs without the runner, after its dependencies,
// and later tasks see the computed values; no second compute at the end.
func TestRunJob_ComputeTaskRunsWithoutRunner(t *testing.T) {
	ctx := context.Background()
	o, f := newTestOrchestrator(t)
	seedDocument(t, f, withTwoPersons)

	computes := 0
	f.Format.ComputeFn = func(*core.Document) core.ComputeResult {
		computes++
		return core.ComputeResult{Fills: []core.Fill{{Path: pathMontantTotal, Value: "100"}}}
	}
	var planSawComputed []bool
	f.Format.OnPlan = func(st core.PlanState) {
		_, ok := st.Doc.Values[pathMontantTotal]
		planSawComputed = append(planSawComputed, ok)
	}
	compute := core.TaskSpec{Kind: core.TaskKindCompute, DependsOnKinds: []string{"fill"}}
	f.Format.Waves = [][]core.TaskSpec{
		{Spec("fill", tgtPP1)},
		{compute},
		{Spec("judgment", tgtPP2, core.TaskKindCompute)},
	}
	f.Runner.On("fill", tgtPP1, Step{Out: outputWith(Fill(pathNomPP1, "DUPONT"))})
	var judgmentSaw string
	f.Runner.On("judgment", tgtPP2, Step{Before: func() {
		v, _ := valueOf(t, f, pathMontantTotal)
		judgmentSaw = v.Value
	}})

	require.NoError(t, o.RunJob(ctx, f.JobId))

	require.Equal(t, []string{key("fill", tgtPP1), key("judgment", tgtPP2)}, f.Runner.Calls,
		"the compute task never goes through the runner")
	require.Equal(t, "100", judgmentSaw, "judgment runs after the computed values were written")
	require.Equal(t, 1, computes, "no second compute at the end of the job")
	v, ok := valueOf(t, f, pathMontantTotal)
	require.True(t, ok)
	require.Equal(t, core.OriginComputed, v.Origin)
	ct := f.Repo.Task(t, core.TaskKindCompute, "")
	require.Equal(t, models.SarJobStatusDone, ct.Status)
	var res taskResult
	require.NoError(t, json.Unmarshal(ct.Output, &res))
	require.Equal(t, []core.Path{pathMontantTotal}, res.Applied)
	require.Equal(t, models.SarJobStatusDone, jobOf(t, f).Status)
	require.Contains(t, planSawComputed, true, "Plan sees the computed values")
}

// A compute task already done is not recomputed on resume.
func TestResume_ComputeTaskNotRerun(t *testing.T) {
	ctx := context.Background()
	o, f := newTestOrchestrator(t)
	seedDocument(t, f, withTwoPersons)
	computes := 0
	f.Format.ComputeFn = func(*core.Document) core.ComputeResult {
		computes++
		return core.ComputeResult{Fills: []core.Fill{{Path: pathMontantTotal, Value: "100"}}}
	}
	f.Format.Waves = [][]core.TaskSpec{
		{Spec("fill", tgtPP1)},
		{{Kind: core.TaskKindCompute, DependsOnKinds: []string{"fill"}}},
		{Spec("judgment", tgtPP2, core.TaskKindCompute)},
	}
	f.Runner.On("judgment", tgtPP2, Step{Err: errProvider}, Step{})
	require.Error(t, o.RunJob(ctx, f.JobId))
	require.NoError(t, o.RunJob(ctx, f.JobId))
	require.Equal(t, 1, computes)
}

// M1: a review job refuses non-review specs.
func TestRunJob_ReviewJobRefusesNonReviewSpecs(t *testing.T) {
	o, f := newTestOrchestrator(t)
	seedDocument(t, f, withTwoPersons)
	f.Repo.jobs[f.JobId].Kind = models.SarJobKindReview
	f.Format.ComputeFn = func(*core.Document) core.ComputeResult {
		return core.ComputeResult{Fills: []core.Fill{{Path: pathMontantTotal, Value: "100"}}}
	}
	f.Format.Waves = [][]core.TaskSpec{{Spec("fill", tgtPP1), Spec(core.TaskKindReview, tgtPP1)}}
	f.Runner.On("fill", tgtPP1, Step{Out: outputWith(Fill(pathNomPP1, "INTRUS"))})

	require.NoError(t, o.RunJob(context.Background(), f.JobId))

	require.Equal(t, []string{key(core.TaskKindReview, tgtPP1)}, f.Runner.Calls)
	require.Equal(t, 0, f.Repo.UpdateCalls)
	require.Equal(t, models.SarJobStatusDone, jobOf(t, f).Status)
}

// M5: a panic after a successful claim still reports the task as claimed.
func TestRunTask_PanicAfterClaimReportsClaimed(t *testing.T) {
	o, f := newTestOrchestrator(t)
	seedDocument(t, f, withTwoPersons)
	spec := Spec("a", tgtPP1)
	task := taskFromSpec(f.JobId, spec, nil)
	require.NoError(t, f.Repo.CreateSarGenerationTasks(context.Background(), nil, []models.SarGenerationTask{task}))
	f.Runner.On("a", tgtPP1, Step{Before: func() { panic("boom") }})
	run := &jobRun{o: o, job: *f.Repo.jobs[f.JobId], declId: f.DeclId, orgId: f.OrgId, format: f.Format}

	claimed, err := run.runTask(context.Background(), readyTask{task: task, spec: spec})
	require.Error(t, err)
	require.True(t, claimed)
}

// M2: the worker stores the River attempt on the job.
func TestWorker_StoresRiverAttempt(t *testing.T) {
	_, f := newTestOrchestrator(t)
	w := NewWorker(workerRunnerStub{}, f.Repo, fakeExecFactory{}, 0)
	require.NoError(t, w.Work(context.Background(), workerJob(f, 3)))
	require.Equal(t, 3, jobOf(t, f).Attempt)
}

// M4: shutdown snoozes without consuming an attempt; a vanished job cancels.
func TestWorker_ShutdownSnoozesAndGoneJobCancels(t *testing.T) {
	o, f := newTestOrchestrator(t)
	w := NewWorker(workerRunnerStub{err: errProvider}, f.Repo, fakeExecFactory{}, 0)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := w.Work(ctx, workerJob(f, models.SarGenerationMaxAttempts))
	var snooze *river.JobSnoozeError
	require.True(t, errors.As(err, &snooze), "got %v", err)
	require.Zero(t, snooze.Duration)
	require.NotEqual(t, models.SarJobStatusFailed, jobOf(t, f).Status, "no attempt consumed, job not failed")

	// A timeout (deadline) is a failure, not a shutdown.
	dctx, dcancel := context.WithTimeout(context.Background(), 0)
	defer dcancel()
	err = w.Work(dctx, workerJob(f, 1))
	require.False(t, errors.As(err, &snooze))
	require.Error(t, err)

	real := NewWorker(o, f.Repo, fakeExecFactory{}, 0)
	job := workerJob(f, 1)
	job.Args.JobId = uuid.New()
	err = real.Work(context.Background(), job)
	var cancelErr *river.JobCancelError
	require.True(t, errors.As(err, &cancelErr), "got %v", err)
}
