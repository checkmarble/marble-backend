package engine

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"
	"github.com/stretchr/testify/require"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/usecases/sargen/core"
)

// Real TRACFIN 1.4.0 paths.
const (
	pathNomPP1       = "personnes.pp[pp-1].etatCivil.nomNaissance" // copied: needs a source
	pathNomPP2       = "personnes.pp[pp-2].etatCivil.nomNaissance"
	pathImplPP1      = "personnes.pp[pp-1].implication" // judgment: justification + confidence
	pathMontantTotal = "operations.montantTotal"        // computed
	implicationValue = "Personne soupçonnée / Personne impliquée"
	tgtPP1           = "personnes.pp[pp-1]"
	tgtPP2           = "personnes.pp[pp-2]"
)

// ---------- helpers (specific to this file) ----------

// seedDocument replaces the declaration document by the one built by mutate
// (nil = empty document).
func seedDocument(t *testing.T, f *fakes, mutate func(d *core.Document)) {
	t.Helper()
	d := core.NewDocument()
	if mutate != nil {
		mutate(d)
	}
	b, err := json.Marshal(d)
	require.NoError(t, err)
	f.Repo.decl.Document = b
}

// withTwoPersons seeds the existing items personnes.pp[pp-1] and personnes.pp[pp-2].
func withTwoPersons(d *core.Document) {
	d.AddItem("personnes.pp", "pp")
	d.AddItem("personnes.pp", "pp")
}

func outputWith(fills ...core.Fill) core.TaskOutput {
	return core.TaskOutput{Fills: fills}
}

func jobOf(t *testing.T, f *fakes) models.SarGenerationJob {
	t.Helper()
	j, err := f.Repo.GetSarGenerationJob(context.Background(), nil, f.JobId)
	require.NoError(t, err)
	return j
}

func questionsOf(t *testing.T, f *fakes, status string) []models.SarQuestion {
	t.Helper()
	qs, err := f.Repo.ListSarQuestions(context.Background(), nil, f.DeclId, status)
	require.NoError(t, err)
	return qs
}

func valueOf(t *testing.T, f *fakes, path string) (core.Value, bool) {
	t.Helper()
	v, ok := f.Repo.Doc(t).Values[core.Path(path)]
	return v, ok
}

// workerJob builds the River job pointing to the fixture job at a given attempt.
func workerJob(f *fakes, attempt int) *river.Job[models.SarGenerationArgs] {
	return &river.Job[models.SarGenerationArgs]{
		JobRow: &rivertype.JobRow{Attempt: attempt, MaxAttempts: models.SarGenerationMaxAttempts},
		Args:   models.SarGenerationArgs{JobId: f.JobId, DeclarationId: f.DeclId},
	}
}

// workerRunnerStub is a JobRunner returning a fixed error.
type workerRunnerStub struct{ err error }

func (s workerRunnerStub) RunJob(context.Context, uuid.UUID) error { return s.err }

// ---------- orchestrator ----------

func TestRunJob_RunsDependenciesInOrder(t *testing.T) {
	ctx := context.Background()
	o, f := newTestOrchestrator(t)
	seedDocument(t, f, withTwoPersons)

	a := Spec("a", tgtPP1)
	b := Spec("b", tgtPP2, "a")
	// b is listed first: it must still wait for a.
	f.Format.Waves = [][]core.TaskSpec{{b, a}}
	f.Runner.On("a", tgtPP1, Step{Out: outputWith(Fill(pathNomPP1, "DUPONT"))})
	f.Runner.On("b", tgtPP2, Step{Out: outputWith(Fill(pathNomPP2, "MARTIN"))})

	require.NoError(t, o.RunJob(ctx, f.JobId))

	require.Equal(t, []string{key("a", tgtPP1), key("b", tgtPP2)}, f.Runner.Calls)
	require.Equal(t, models.SarJobStatusDone, jobOf(t, f).Status)
	require.Equal(t, models.SarJobStatusDone, f.Repo.Task(t, "a", tgtPP1).Status)
	require.Equal(t, models.SarJobStatusDone, f.Repo.Task(t, "b", tgtPP2).Status)
}

func TestResume_DoesNotReapplyDoneTask(t *testing.T) {
	ctx := context.Background()
	o, f := newTestOrchestrator(t)
	seedDocument(t, f, nil)

	// A creates the first personne (provisional key), B completes it.
	a := Spec("a", "personnes.pp")
	b := Spec("b", tgtPP1, "a")
	f.Format.Waves = [][]core.TaskSpec{{a}, {b}}
	f.Runner.On("a", "personnes.pp", Step{Out: outputWith(Fill("personnes.pp[+a].etatCivil.nomNaissance", "DUPONT"))})
	f.Runner.On("b", tgtPP1,
		Step{Err: errProvider}, // technical error on the first pass
		Step{Out: outputWith(Fill(pathImplPP1, implicationValue))},
	)

	// First pass: A done, B fails, RunJob returns the technical error for River.
	err := o.RunJob(ctx, f.JobId)
	require.Error(t, err)
	require.True(t, errors.Is(err, errProvider))
	require.Equal(t, models.SarJobStatusRunning, jobOf(t, f).Status)
	require.Equal(t, models.SarJobStatusDone, f.Repo.Task(t, "a", "personnes.pp").Status)
	require.Equal(t, models.SarJobStatusFailed, f.Repo.Task(t, "b", tgtPP1).Status)

	// Resume: A is not run again, B is.
	require.NoError(t, o.RunJob(ctx, f.JobId))

	require.Equal(t, 1, f.Runner.Count("a", "personnes.pp"))
	require.Equal(t, 2, f.Runner.Count("b", tgtPP1))
	// A's personne exists exactly once: a re-application would have created pp-2.
	doc := f.Repo.Doc(t)
	require.Equal(t, []string{"pp-1"}, doc.ItemKeys("personnes.pp"))
	v, ok := valueOf(t, f, pathNomPP1)
	require.True(t, ok)
	require.Equal(t, "DUPONT", v.Value)
	require.Equal(t, models.SarJobStatusDone, jobOf(t, f).Status)
}

func TestResume_StaleRunningTaskIsRetried(t *testing.T) {
	ctx := context.Background()
	o, f := newTestOrchestrator(t)
	seedDocument(t, f, withTwoPersons)

	a := Spec("a", tgtPP1)
	f.Format.Waves = [][]core.TaskSpec{{a}}
	f.Runner.On("a", tgtPP1,
		Step{Err: errProvider},
		Step{Out: outputWith(Fill(pathNomPP1, "DUPONT"))},
	)
	require.Error(t, o.RunJob(ctx, f.JobId))

	// The worker died mid-task: the task is still running, started 2h ago
	// (StaleAfter is 1h in the fixture).
	f.Repo.SetTask(t, "a", tgtPP1, func(x *models.SarGenerationTask) {
		x.Status = models.SarJobStatusRunning
		started := f.Now().Add(-2 * time.Hour)
		x.StartedAt = &started
	})

	require.NoError(t, o.RunJob(ctx, f.JobId))

	require.Equal(t, 2, f.Runner.Count("a", tgtPP1))
	require.Equal(t, models.SarJobStatusDone, f.Repo.Task(t, "a", tgtPP1).Status)
	v, ok := valueOf(t, f, pathNomPP1)
	require.True(t, ok)
	require.Equal(t, "DUPONT", v.Value)
}

func TestRunJob_TaskCannotWriteOutsideTarget(t *testing.T) {
	ctx := context.Background()
	o, f := newTestOrchestrator(t)
	seedDocument(t, f, withTwoPersons)

	a := Spec("a", tgtPP1)
	f.Format.Waves = [][]core.TaskSpec{{a}}
	f.Runner.On("a", tgtPP1, Step{Out: outputWith(
		Fill(pathNomPP1, "DUPONT"),
		Fill(pathNomPP2, "INTRUS"), // pp-2 is not the target of the task
	)})

	require.NoError(t, o.RunJob(ctx, f.JobId))

	_, ok := valueOf(t, f, pathNomPP2)
	require.False(t, ok, "a fill outside the target must not be written")
	v, ok := valueOf(t, f, pathNomPP1)
	require.True(t, ok)
	require.Equal(t, "DUPONT", v.Value)
}

func TestApplyTaskOutput_ConcurrentManualEdit(t *testing.T) {
	ctx := context.Background()
	o, f := newTestOrchestrator(t)
	seedDocument(t, f, withTwoPersons)

	a := Spec("a", tgtPP1)
	f.Format.Waves = [][]core.TaskSpec{{a}}
	f.Runner.On("a", tgtPP1, Step{
		Out: outputWith(Fill(pathNomPP1, "LLM")),
		// The analyst types a value while the task runs, before its output is applied.
		Before: func() { f.Repo.ManualEdit(t, core.Path(pathNomPP1), "MANUAL") },
	})

	require.NoError(t, o.RunJob(ctx, f.JobId))

	v, ok := valueOf(t, f, pathNomPP1)
	require.True(t, ok)
	require.Equal(t, "MANUAL", v.Value, "the manual value must be kept")
	require.Equal(t, core.OriginManual, v.Origin)

	task := f.Repo.Task(t, "a", tgtPP1)
	require.Equal(t, models.SarJobStatusDone, task.Status)
	out := strings.ToLower(string(task.Output))
	require.Contains(t, out, "conflict", "the refused fill must be kept as a conflict in the task output")
	require.Contains(t, string(task.Output), pathNomPP1)
}

func TestRunJob_RateLimitSnoozes(t *testing.T) {
	ctx := context.Background()
	o, f := newTestOrchestrator(t)

	a := Spec("a", tgtPP1)
	f.Format.Waves = [][]core.TaskSpec{{a}}
	f.Runner.On("a", tgtPP1, Step{Err: RateLimited()})

	// RunJob keeps the rate limit detectable.
	err := o.RunJob(ctx, f.JobId)
	require.True(t, errors.Is(err, models.LLMRateLimitedError))
	require.Equal(t, models.SarJobStatusRunning, jobOf(t, f).Status)

	// The worker turns it into a snooze, without counting an attempt.
	w := NewWorker(o, f.Repo, fakeExecFactory{}, time.Minute)
	for _, attempt := range []int{1, models.SarGenerationMaxAttempts} {
		err := w.Work(ctx, workerJob(f, attempt))
		var snooze *river.JobSnoozeError
		require.True(t, errors.As(err, &snooze), "attempt %d: expected a snooze, got %v", attempt, err)
		require.Equal(t, RateLimitSnooze, snooze.Duration)
		require.NotEqual(t, models.SarJobStatusFailed, jobOf(t, f).Status,
			"a rate limit on the last attempt must not fail the job")
	}
}

func TestRunJob_TechnicalFailureReturnsErrorForRiverRetry(t *testing.T) {
	ctx := context.Background()
	o, f := newTestOrchestrator(t)
	seedDocument(t, f, withTwoPersons)

	a := Spec("a", tgtPP1)
	b := Spec("b", tgtPP1, "a")
	c := Spec("c", tgtPP2, "a")
	f.Format.Waves = [][]core.TaskSpec{{a}, {b, c}}
	f.Runner.On("a", tgtPP1, Step{Out: outputWith(Fill(pathNomPP1, "DUPONT"))})
	f.Runner.On("b", tgtPP1, Step{Err: errProvider})
	f.Runner.On("c", tgtPP2, Step{Out: outputWith(Fill(pathNomPP2, "MARTIN"))})

	err := o.RunJob(ctx, f.JobId)
	require.Error(t, err)
	require.True(t, errors.Is(err, errProvider))

	// The job stays running for River to retry; the worker marks it failed on the last attempt.
	require.Equal(t, models.SarJobStatusRunning, jobOf(t, f).Status)
	require.Equal(t, models.SarJobStatusDone, f.Repo.Task(t, "a", tgtPP1).Status)
	require.Equal(t, models.SarJobStatusFailed, f.Repo.Task(t, "b", tgtPP1).Status)
	// The task in flight alongside the failing one still finishes.
	require.Equal(t, models.SarJobStatusDone, f.Repo.Task(t, "c", tgtPP2).Status)
	_, ok := valueOf(t, f, pathNomPP2)
	require.True(t, ok)
}

func TestTaskQuestionsArePersistedWithoutDuplicates(t *testing.T) {
	ctx := context.Background()
	o, f := newTestOrchestrator(t)
	seedDocument(t, f, withTwoPersons)

	question := core.QuestionDraft{
		Paths: []core.Path{pathImplPP1},
		Text:  "Quelle est l'implication de cette personne ?",
	}
	a := Spec("a", tgtPP1)
	b := Spec("b", tgtPP1)
	f.Format.Waves = [][]core.TaskSpec{{a}, {b}}
	f.Runner.On("a", tgtPP1, Step{Out: core.TaskOutput{Questions: []core.QuestionDraft{question}}})
	f.Runner.On("b", tgtPP1, Step{
		Out: core.TaskOutput{Questions: []core.QuestionDraft{question}},
		// The question raised by A is closed before B asks it again.
		Before: func() {
			var ids []uuid.UUID
			for _, q := range questionsOf(t, f, "") {
				ids = append(ids, q.Id)
			}
			require.NoError(t, f.Repo.CloseSarQuestions(ctx, nil, ids, models.SarQuestionStatusResolved))
		},
	})

	require.NoError(t, o.RunJob(ctx, f.JobId))

	qs := questionsOf(t, f, "")
	require.Len(t, qs, 1, "same paths and text must not be persisted twice, closed ones included")
	require.Equal(t, models.SarQuestionStatusResolved, qs[0].Status)
	require.Equal(t, []string{pathImplPP1}, qs[0].Paths)
}

func TestReviewTaskNeverWritesDocument(t *testing.T) {
	ctx := context.Background()
	o, f := newTestOrchestrator(t)
	seedDocument(t, f, withTwoPersons)

	// A finding raised by a previous review.
	require.NoError(t, f.Repo.CreateSarQuestions(ctx, nil, []models.SarQuestion{{
		Id: uuid.New(), DeclarationId: f.DeclId, Kind: models.SarQuestionKindReviewFinding,
		Severity: "mineur", Paths: []string{pathNomPP1}, Text: "ancien constat",
		Status: models.SarQuestionStatusOpen, CreatedAt: f.Now(),
	}}))

	review := Spec(core.TaskKindReview, tgtPP1)
	f.Format.Waves = [][]core.TaskSpec{{review}}
	f.Runner.On(core.TaskKindReview, tgtPP1, Step{Out: core.TaskOutput{
		// A review never writes: this fill must be ignored.
		Fills: []core.Fill{Fill(pathNomPP1, "INTRUS")},
		Findings: []core.ReviewFinding{{
			Paths: []core.Path{pathNomPP1}, Severity: "important", Message: "Nom à vérifier",
		}},
	}})

	require.NoError(t, o.RunJob(ctx, f.JobId))

	require.Equal(t, 1, f.Repo.decl.Version, "the document version must not change")
	require.Equal(t, 0, f.Repo.UpdateCalls)
	_, ok := valueOf(t, f, pathNomPP1)
	require.False(t, ok)

	open := questionsOf(t, f, models.SarQuestionStatusOpen)
	require.Len(t, open, 1)
	require.Equal(t, models.SarQuestionKindReviewFinding, open[0].Kind)
	require.Equal(t, "Nom à vérifier", open[0].Text)
	require.Equal(t, "important", open[0].Severity)

	for _, q := range questionsOf(t, f, "") {
		if q.Text == "ancien constat" {
			require.NotEqual(t, models.SarQuestionStatusOpen, q.Status, "the previous open finding must be replaced")
		}
	}
}

func TestRunJob_StampsOriginAndTaskIdOnFills(t *testing.T) {
	ctx := context.Background()
	o, f := newTestOrchestrator(t)
	seedDocument(t, f, withTwoPersons)

	a := Spec("a", tgtPP1)
	f.Format.Waves = [][]core.TaskSpec{{a}}
	fill := Fill(pathNomPP1, "DUPONT")
	fill.Origin = core.OriginManual // the agent claims the manual origin, and gives no task id
	f.Runner.On("a", tgtPP1, Step{Out: outputWith(fill)})

	require.NoError(t, o.RunJob(ctx, f.JobId))

	v, ok := valueOf(t, f, pathNomPP1)
	require.True(t, ok)
	require.Equal(t, core.OriginLLM, v.Origin)
	require.NotNil(t, v.TaskID)
	require.Equal(t, f.Repo.Task(t, "a", tgtPP1).Id, *v.TaskID)
}

func TestRunJob_ComputeOnlyAfterAllFillTasks(t *testing.T) {
	ctx := context.Background()
	o, f := newTestOrchestrator(t)
	seedDocument(t, f, withTwoPersons)

	var mu sync.Mutex
	var events []string
	record := func(e string) {
		mu.Lock()
		defer mu.Unlock()
		events = append(events, e)
	}

	f.Format.ComputeFn = func(*core.Document) core.ComputeResult {
		record("compute")
		return core.ComputeResult{Fills: []core.Fill{{Path: pathMontantTotal, Value: "100"}}}
	}
	a := Spec("a", tgtPP1)
	b := Spec("b", tgtPP2)
	f.Format.Waves = [][]core.TaskSpec{{a}, {b}}
	f.Runner.On("a", tgtPP1, Step{Out: outputWith(Fill(pathNomPP1, "DUPONT")), Before: func() { record("run:a") }})
	f.Runner.On("b", tgtPP2, Step{Out: outputWith(Fill(pathNomPP2, "MARTIN")), Before: func() { record("run:b") }})

	require.NoError(t, o.RunJob(ctx, f.JobId))

	require.Equal(t, []string{"run:a", "run:b", "compute"}, events,
		"compute must run once, after every fill task is done")
	v, ok := valueOf(t, f, pathMontantTotal)
	require.True(t, ok)
	require.Equal(t, "100", v.Value)
	require.Equal(t, core.OriginComputed, v.Origin)
}

func TestRunJob_QuestionPathsRewrittenFromProvisionalKeys(t *testing.T) {
	ctx := context.Background()
	o, f := newTestOrchestrator(t)
	seedDocument(t, f, nil)

	const provisional = "personnes.pp[+a].etatCivil.nomNaissance"
	a := Spec("a", "personnes.pp")
	f.Format.Waves = [][]core.TaskSpec{{a}}
	f.Runner.On("a", "personnes.pp", Step{Out: core.TaskOutput{
		Fills: []core.Fill{Fill(provisional, "DUPONT")},
		Questions: []core.QuestionDraft{{
			Paths: []core.Path{core.Path(provisional)},
			Text:  "Confirmer le nom de naissance ?",
		}},
	}})

	require.NoError(t, o.RunJob(ctx, f.JobId))

	items := f.Repo.Doc(t).ItemKeys("personnes.pp")
	require.Len(t, items, 1)
	real := "personnes.pp[" + items[0] + "].etatCivil.nomNaissance"
	_, ok := valueOf(t, f, real)
	require.True(t, ok)

	qs := questionsOf(t, f, "")
	require.Len(t, qs, 1)
	require.Equal(t, []string{real}, qs[0].Paths, "the question must cite the real item key, not [+a]")
}

func TestRunJob_SupersededAttemptOutputDiscarded(t *testing.T) {
	ctx := context.Background()
	o, f := newTestOrchestrator(t)
	seedDocument(t, f, withTwoPersons)

	a := Spec("a", tgtPP1)
	f.Format.Waves = [][]core.TaskSpec{{a}}
	f.Runner.On("a", tgtPP1, Step{
		Out: outputWith(Fill(pathNomPP1, "DUPONT")),
		// Another worker took over and finished the task while this run was in flight.
		Before: func() {
			f.Repo.SetTask(t, "a", tgtPP1, func(x *models.SarGenerationTask) {
				x.Status = models.SarJobStatusDone
				x.Attempts++
				x.Output = nil
			})
		},
	})

	// The superseded result is discarded, which is not a technical failure.
	require.NoError(t, o.RunJob(ctx, f.JobId))

	_, ok := valueOf(t, f, pathNomPP1)
	require.False(t, ok, "the output of a superseded attempt must not be applied")
	require.Equal(t, 1, f.Repo.decl.Version)
}

func TestRunJob_MissingPromptFailsJob(t *testing.T) {
	ctx := context.Background()
	o, f := newTestOrchestrator(t)
	seedDocument(t, f, withTwoPersons)

	a := Spec("a", tgtPP1)
	f.Format.Waves = [][]core.TaskSpec{{a}}
	f.Runner.On("a", tgtPP1, Step{Out: outputWith(Fill(pathNomPP1, "DUPONT"))})
	f.Prompts.Missing["prompts/a.md"] = true

	// The returned error is not part of the contract here: the job state is.
	_ = o.RunJob(ctx, f.JobId)

	j := jobOf(t, f)
	require.Equal(t, models.SarJobStatusFailed, j.Status)
	require.Contains(t, j.Error, "prompt manquant")
	require.Contains(t, j.Error, "prompts/a.md")
	require.Equal(t, 0, f.Runner.Count("a", tgtPP1), "no task may run when a prompt is missing")
}

// ---------- worker ----------

func TestWorker_LastAttemptMarksJobFailed(t *testing.T) {
	ctx := context.Background()
	_, f := newTestOrchestrator(t)
	w := NewWorker(workerRunnerStub{err: errProvider}, f.Repo, fakeExecFactory{}, time.Minute)

	// Not the last attempt: River retries, the job is left as it was.
	require.Error(t, w.Work(ctx, workerJob(f, 1)))
	require.Equal(t, models.SarJobStatusPending, jobOf(t, f).Status)

	// Last attempt: the job is failed before the error is returned.
	require.Error(t, w.Work(ctx, workerJob(f, models.SarGenerationMaxAttempts)))
	j := jobOf(t, f)
	require.Equal(t, models.SarJobStatusFailed, j.Status)
	require.NotEmpty(t, j.Error)
}

func TestQuestionOptionFillsAreStampedWithLLMOrigin(t *testing.T) {
	ctx := context.Background()
	o, f := newTestOrchestrator(t)
	seedDocument(t, f, withTwoPersons)
	a := Spec("a", tgtPP1)
	f.Format.Waves = [][]core.TaskSpec{{a}}
	f.Runner.On("a", tgtPP1, Step{Out: core.TaskOutput{Questions: []core.QuestionDraft{{
		Paths: []core.Path{pathImplPP1}, Text: "Implication ?",
		Options: []core.QuestionOption{{Label: "x", Fills: []core.Fill{{Path: pathImplPP1, Value: "v", Origin: core.OriginOrgSetting}}}},
	}}}})
	require.NoError(t, o.RunJob(ctx, f.JobId))

	qs := questionsOf(t, f, "")
	require.Len(t, qs, 1)
	var opts []core.QuestionOption
	require.NoError(t, json.Unmarshal(qs[0].Options, &opts))
	require.Equal(t, core.OriginLLM, opts[0].Fills[0].Origin, "the model never picks the origin")
	require.NotNil(t, opts[0].Fills[0].TaskID)
}

// A task output records whether it changed the document: the selection stays
// editable after a failed job whose done tasks all wrote nothing.
func TestRunJob_TaskOutputRecordsWhetherTheDocumentChanged(t *testing.T) {
	ctx := context.Background()
	o, f := newTestOrchestrator(t)
	seedDocument(t, f, withTwoPersons)

	writer := Spec("a", tgtPP1)
	silent := Spec("b", tgtPP2)
	f.Format.Waves = [][]core.TaskSpec{{writer, silent}}
	f.Runner.On("a", tgtPP1, Step{Out: outputWith(Fill(pathNomPP1, "DUPONT"))})
	f.Runner.On("b", tgtPP2, Step{Out: core.TaskOutput{}})

	require.NoError(t, o.RunJob(ctx, f.JobId))

	wrote := func(task models.SarGenerationTask) bool {
		var out struct {
			Wrote bool `json:"wrote"`
		}
		require.NoError(t, json.Unmarshal(task.Output, &out))
		return out.Wrote
	}
	require.True(t, wrote(f.Repo.Task(t, "a", tgtPP1)))
	require.False(t, wrote(f.Repo.Task(t, "b", tgtPP2)))
}

// A failed task whose spec is no longer planned when the job resumes (the
// analysis changed in between) is deleted and never runs: only the task of the
// current plan runs, so its output is applied once.
func TestResume_StaleNotDoneTaskIsDeletedNotRun(t *testing.T) {
	ctx := context.Background()
	o, f := newTestOrchestrator(t)
	seedDocument(t, f, withTwoPersons)

	old := Spec("a", tgtPP1)
	old.Payload = map[string]any{"analysis": "first text"}
	f.Format.Waves = [][]core.TaskSpec{{old}}
	f.Runner.On("a", tgtPP1, Step{Err: errProvider})
	require.Error(t, o.RunJob(ctx, f.JobId))
	require.Equal(t, 1, f.Runner.Count("a", tgtPP1))

	// The analyst edits the analysis: the plan now carries another payload.
	cur := Spec("a", tgtPP1)
	cur.Payload = map[string]any{"analysis": "edited text"}
	f.Format.Waves = [][]core.TaskSpec{{cur}}
	f.Runner.On("a", tgtPP1, Step{Out: outputWith(Fill(pathNomPP1, "DUPONT"))})

	require.NoError(t, o.RunJob(ctx, f.JobId))

	require.Equal(t, 2, f.Runner.Count("a", tgtPP1), "one run for the failed task, one for the current plan, none for the stale task")
	tasks, err := f.Repo.ListSarGenerationTasks(ctx, nil, f.JobId)
	require.NoError(t, err)
	require.Len(t, tasks, 1, "the stale task is deleted")
	require.Equal(t, models.SarJobStatusDone, tasks[0].Status)
	require.Contains(t, string(tasks[0].Input), "edited text")
}
