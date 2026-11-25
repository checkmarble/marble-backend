package integration

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"
	"github.com/stretchr/testify/require"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/sargen/core"
	"github.com/checkmarble/marble-backend/usecases/sargen/engine"
	"github.com/checkmarble/marble-backend/usecases/sargen/formats"
)

// The orchestrator runs here against the real database and the real
// transaction factory; only the LLM runner and the planning are scripted.

const (
	engPP1Name = "personnes.pp[pp-1].etatCivil.nomNaissance"
	engPP1Impl = "personnes.pp[pp-1].implication"
	engPP1     = "personnes.pp[pp-1]"
)

// engFormat is the real TRACFIN spec with a scripted plan.
type engFormat struct {
	core.Format
	waves [][]core.TaskSpec
}

func (f *engFormat) Plan(st core.PlanState) []core.TaskSpec {
	done := map[string]bool{}
	for _, t := range st.Done {
		done[t.Kind+"|"+t.TargetPath] = true
	}
	for _, wave := range f.waves {
		for _, s := range wave {
			if !done[s.Kind+"|"+string(s.TargetPath)] {
				return wave
			}
		}
	}
	return nil
}

func (f *engFormat) Compute(*core.Document) core.ComputeResult { return core.ComputeResult{} }

type engRunner struct {
	mu    sync.Mutex
	runs  map[string]int
	steps map[string]func(attempt int) (core.TaskOutput, error)
}

func (r *engRunner) Run(_ context.Context, _ core.ToolContext, task models.SarGenerationTask, _ core.TaskSpec) (core.TaskOutput, error) {
	k := task.Kind
	r.mu.Lock()
	r.runs[k]++
	step := r.steps[k]
	r.mu.Unlock()
	if step == nil {
		return core.TaskOutput{}, nil
	}
	return step(task.Attempts)
}

func (r *engRunner) count(kind string) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.runs[kind]
}

type engPrompts struct{}

func (engPrompts) Exists(string) bool { return true }

type engDataModels struct{}

func (engDataModels) GetDataModel(context.Context, uuid.UUID, models.DataModelReadOptions, bool) (models.DataModel, error) {
	return models.DataModel{}, nil
}

type engHarness struct {
	repo   *repositories.MarbleDbRepository
	exec   repositories.Executor
	decl   models.SarDeclaration
	job    models.SarGenerationJob
	format *engFormat
	runner *engRunner
	orch   *engine.Orchestrator
}

func engFill(path, value string) core.Fill {
	return core.Fill{
		Path: core.Path(path), Value: value, Origin: core.OriginLLM, Confidence: core.ConfidenceHigh,
		Justification: "test",
		Sources:       []core.SourceRef{{Object: core.ObjectRef{Table: "t", ObjectID: "o"}, Field: "f", RawValue: "v"}},
	}
}

func engSpec(kind, target string, dependsOn ...string) core.TaskSpec {
	return core.TaskSpec{Kind: kind, TargetPath: core.Path(target), Tier: core.TierLight, DependsOnKinds: dependsOn}
}

// newEngHarness creates a declaration holding one personne (pp-1) and a
// pending generate job.
func newEngHarness(t *testing.T, staleAfter time.Duration) *engHarness {
	t.Helper()
	ctx := t.Context()
	orgId, caseId := sarDeclarationTestFixture(t)
	repo := repositories.NewMarbleDbRepository(false, 0)
	exec := testUsecases.NewExecutorFactory().NewExecutor()

	doc := core.NewDocument()
	doc.AddItem("personnes.pp", "pp")
	raw, err := json.Marshal(doc)
	require.NoError(t, err)
	decl := newSarTestDeclaration(orgId, caseId)
	decl.Document = raw
	require.NoError(t, repo.CreateSarDeclaration(ctx, exec, decl))
	job := newSarGenTestJob(decl.Id)
	require.NoError(t, repo.CreateSarGenerationJob(ctx, exec, job))

	base, ok := formats.Get(core.FormatKey{Family: "tracfin", Country: "FR", Version: "1.4.0"})
	require.True(t, ok)
	h := &engHarness{
		repo: repo, exec: exec, decl: decl, job: job,
		format: &engFormat{Format: base},
		runner: &engRunner{runs: map[string]int{}, steps: map[string]func(int) (core.TaskOutput, error){}},
	}
	h.orch = engine.NewOrchestrator(engine.Deps{
		ExecutorFactory:    testUsecases.NewExecutorFactory(),
		TransactionFactory: testUsecases.NewTransactionFactory(),
		Declarations:       repo, Selection: repo, Generation: repo,
		DataModels: engDataModels{}, Runner: h.runner, Prompts: engPrompts{},
		FormatGetter: func(core.FormatKey) (core.Format, bool) { return h.format, true },
		Parallelism:  1, StaleAfter: staleAfter,
	})
	return h
}

func (h *engHarness) declaration(t *testing.T) (models.SarDeclaration, *core.Document) {
	t.Helper()
	d, err := h.repo.GetSarDeclaration(t.Context(), h.exec, h.decl.Id)
	require.NoError(t, err)
	doc := core.NewDocument()
	require.NoError(t, json.Unmarshal(d.Document, doc))
	return d, doc
}

func (h *engHarness) task(t *testing.T, kind string) models.SarGenerationTask {
	t.Helper()
	tasks, err := h.repo.ListSarGenerationTasks(t.Context(), h.exec, h.job.Id)
	require.NoError(t, err)
	for _, tk := range tasks {
		if tk.Kind == kind {
			return tk
		}
	}
	require.FailNow(t, "task not found", kind)
	return models.SarGenerationTask{}
}

func (h *engHarness) jobStatus(t *testing.T) models.SarGenerationJob {
	t.Helper()
	j, err := h.repo.GetSarGenerationJob(t.Context(), h.exec, h.job.Id)
	require.NoError(t, err)
	return j
}

func (h *engHarness) questions(t *testing.T) []models.SarQuestion {
	t.Helper()
	qs, err := h.repo.ListSarQuestions(t.Context(), h.exec, h.decl.Id, "")
	require.NoError(t, err)
	return qs
}

func TestSarEngine_NormalWaveAppliesAndCompletes(t *testing.T) {
	h := newEngHarness(t, time.Hour)
	h.format.waves = [][]core.TaskSpec{{engSpec("a", engPP1)}}
	h.runner.steps["a"] = func(int) (core.TaskOutput, error) {
		return core.TaskOutput{
			Fills:     []core.Fill{engFill(engPP1Name, "DUPONT")},
			Questions: []core.QuestionDraft{{Paths: []core.Path{engPP1Impl}, Text: "Implication ?"}},
		}, nil
	}

	require.NoError(t, h.orch.RunJob(t.Context(), h.job.Id))

	d, doc := h.declaration(t)
	require.Equal(t, 2, d.Version, "one document write")
	require.Equal(t, "DUPONT", doc.Values[engPP1Name].Value)
	require.Equal(t, core.OriginLLM, doc.Values[engPP1Name].Origin)
	tk := h.task(t, "a")
	require.Equal(t, models.SarJobStatusDone, tk.Status)
	require.Equal(t, 1, tk.Attempts)
	require.Equal(t, models.SarJobStatusDone, h.jobStatus(t).Status)
	qs := h.questions(t)
	require.Len(t, qs, 1)
	require.Equal(t, []string{engPP1Impl}, qs[0].Paths)
	require.Equal(t, &tk.Id, qs[0].TaskId)
}

// A failure after Complete, inside the apply transaction, rolls everything
// back: the task is failed (claimable), the document and questions untouched.
func TestSarEngine_FailureAfterCompleteRollsBack(t *testing.T) {
	h := newEngHarness(t, time.Hour)
	h.format.waves = [][]core.TaskSpec{{engSpec("a", engPP1)}}
	h.runner.steps["a"] = func(attempt int) (core.TaskOutput, error) {
		text := "Implication ?"
		if attempt == 1 {
			text = "invalide \x00 pour Postgres" // rejected by the questions insert
		}
		return core.TaskOutput{
			Fills:     []core.Fill{engFill(engPP1Name, "DUPONT")},
			Questions: []core.QuestionDraft{{Paths: []core.Path{engPP1Impl}, Text: text}},
		}, nil
	}

	err := h.orch.RunJob(t.Context(), h.job.Id)
	require.Error(t, err)

	d, doc := h.declaration(t)
	require.Equal(t, 1, d.Version, "document version unchanged")
	require.Empty(t, doc.Values)
	require.Empty(t, h.questions(t))
	tk := h.task(t, "a")
	require.Equal(t, models.SarJobStatusFailed, tk.Status, "failed, hence claimable, not stuck running")
	require.NotEmpty(t, tk.Error)
	require.Equal(t, models.SarJobStatusRunning, h.jobStatus(t).Status)

	// The next attempt reclaims it and applies it once.
	require.NoError(t, h.orch.RunJob(t.Context(), h.job.Id))
	d, doc = h.declaration(t)
	require.Equal(t, 2, d.Version)
	require.Equal(t, "DUPONT", doc.Values[engPP1Name].Value)
	require.Len(t, h.questions(t), 1)
	require.Equal(t, 2, h.task(t, "a").Attempts)
}

// A worker superseded while running (its task reclaimed as stale) must apply
// nothing.
func TestSarEngine_SupersededAttemptAppliesNothing(t *testing.T) {
	h := newEngHarness(t, time.Hour)
	h.format.waves = [][]core.TaskSpec{{engSpec("a", engPP1)}}
	h.runner.steps["a"] = func(int) (core.TaskOutput, error) {
		// Meanwhile another worker finds the task stale and claims it.
		tk := h.task(t, "a")
		_, err := pgPool.Exec(context.Background(),
			"UPDATE sar_generation_tasks SET started_at = now() - interval '3 hours' WHERE id = $1", tk.Id)
		require.NoError(t, err)
		attempt, ok, err := h.repo.ClaimSarGenerationTask(context.Background(), h.exec, tk.Id, time.Hour)
		require.NoError(t, err)
		require.True(t, ok)
		require.Equal(t, 2, attempt)
		return core.TaskOutput{
			Fills:     []core.Fill{engFill(engPP1Name, "DUPONT")},
			Questions: []core.QuestionDraft{{Paths: []core.Path{engPP1Impl}, Text: "Implication ?"}},
		}, nil
	}

	// The first worker finishes without error (its result is discarded); the
	// job loop then sees the task running elsewhere and snoozes.
	err := h.orch.RunJob(t.Context(), h.job.Id)
	require.ErrorIs(t, err, engine.ErrTasksRunningElsewhere)

	d, doc := h.declaration(t)
	require.Equal(t, 1, d.Version)
	require.Empty(t, doc.Values)
	require.Empty(t, h.questions(t))
	tk := h.task(t, "a")
	require.Equal(t, models.SarJobStatusRunning, tk.Status, "still owned by the second attempt")
	require.Equal(t, 2, tk.Attempts)
}

// The PR demo chain: a job cut midway (task 2 fails) is picked up by the next
// River attempt, which finishes it without re-running or re-applying task 1.
func TestSarEngine_CutMidwayThenResumed(t *testing.T) {
	h := newEngHarness(t, time.Hour)
	// Task 1 creates a second personne (provisional key): applying it twice
	// would leave three persons.
	const provisionalName = "personnes.pp[+n].etatCivil.nomNaissance"
	h.format.waves = [][]core.TaskSpec{
		{engSpec("create", "personnes.pp")},
		{engSpec("complete", engPP1, "create")},
	}
	h.runner.steps["create"] = func(int) (core.TaskOutput, error) {
		return core.TaskOutput{Fills: []core.Fill{engFill(provisionalName, "MARTIN")}}, nil
	}
	h.runner.steps["complete"] = func(attempt int) (core.TaskOutput, error) {
		if attempt == 1 {
			return core.TaskOutput{}, context.DeadlineExceeded // cut
		}
		return core.TaskOutput{Fills: []core.Fill{engFill(engPP1Name, "DUPONT")}}, nil
	}

	w := engine.NewWorker(h.orch, h.repo, testUsecases.NewExecutorFactory(), time.Minute)
	riverJob := func(attempt int) *river.Job[models.SarGenerationArgs] {
		return &river.Job[models.SarGenerationArgs]{
			JobRow: &rivertype.JobRow{Attempt: attempt, MaxAttempts: models.SarGenerationMaxAttempts},
			Args:   models.SarGenerationArgs{JobId: h.job.Id, DeclarationId: h.decl.Id},
		}
	}

	require.Error(t, w.Work(t.Context(), riverJob(1)))
	j := h.jobStatus(t)
	require.Equal(t, models.SarJobStatusRunning, j.Status, "left for River to retry")
	require.Equal(t, 1, j.Attempt)
	require.Equal(t, models.SarJobStatusDone, h.task(t, "create").Status)
	require.Equal(t, models.SarJobStatusFailed, h.task(t, "complete").Status)
	_, doc := h.declaration(t)
	require.Len(t, doc.ItemKeys("personnes.pp"), 2)

	require.NoError(t, w.Work(t.Context(), riverJob(2)))

	j = h.jobStatus(t)
	require.Equal(t, models.SarJobStatusDone, j.Status)
	require.Equal(t, 2, j.Attempt)
	d, doc := h.declaration(t)
	require.Len(t, doc.ItemKeys("personnes.pp"), 2, "task 1 applied exactly once")
	require.Equal(t, "DUPONT", doc.Values[engPP1Name].Value)
	created := 0
	for _, v := range doc.Values {
		if v.Value == "MARTIN" {
			created++
		}
	}
	require.Equal(t, 1, created, "task 1 value present exactly once")
	require.Equal(t, 1, h.runner.count("create"), "task 1 not re-run")
	require.Equal(t, 2, h.runner.count("complete"))
	require.Equal(t, 1, h.task(t, "create").Attempts)
	require.Equal(t, 3, d.Version, "one write per task")
}
