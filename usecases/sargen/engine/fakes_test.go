package engine

import (
	"context"
	"encoding/json"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/checkmarble/marble-backend/mocks"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/sargen/core"
	"github.com/checkmarble/marble-backend/usecases/sargen/formats"
)

const testFormatKey = "tracfin:FR:1.4.0"

// ---------- fake TaskRunner ----------

// Step is one scripted run of a task.
type Step struct {
	Out core.TaskOutput
	Err error
	// Before runs inside Run, before returning (simulate a manual edit between
	// Run and apply: call f.Repo.ManualEdit there).
	Before func()
}

// fakeRunner returns scripted steps per (kind, target). Each call consumes the
// next step; the last step repeats; no script = empty output.
type fakeRunner struct {
	mu     sync.Mutex
	script map[string][]Step
	calls  map[string]int
	specs  map[string]core.TaskSpec
	// Calls lists the runs in order (kind|target).
	Calls []string
	// TCs are the tool contexts of the runs, in order.
	TCs []core.ToolContext
}

func newFakeRunner() *fakeRunner {
	return &fakeRunner{script: map[string][]Step{}, calls: map[string]int{}, specs: map[string]core.TaskSpec{}}
}

func key(kind, target string) string { return kind + "|" + target }

// On scripts the steps of a task.
func (r *fakeRunner) On(kind, target string, steps ...Step) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.script[key(kind, target)] = steps
}

// Count returns how many times the task was run.
func (r *fakeRunner) Count(kind, target string) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.calls[key(kind, target)]
}

// LastSpec returns the spec the task was last run with.
func (r *fakeRunner) LastSpec(kind, target string) core.TaskSpec {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.specs[key(kind, target)]
}

func (r *fakeRunner) Run(_ context.Context, tc core.ToolContext, task models.SarGenerationTask, spec core.TaskSpec) (core.TaskOutput, error) {
	k := key(task.Kind, task.TargetPath)
	r.mu.Lock()
	r.specs[k] = spec
	steps := r.script[k]
	n := r.calls[k]
	r.calls[k]++
	r.Calls = append(r.Calls, k)
	r.TCs = append(r.TCs, tc)
	r.mu.Unlock()
	if len(steps) == 0 {
		return core.TaskOutput{}, nil
	}
	if n >= len(steps) {
		n = len(steps) - 1
	}
	if steps[n].Before != nil {
		steps[n].Before()
	}
	return steps[n].Out, steps[n].Err
}

// ---------- fake Format ----------

// fakeFormat wraps the real TRACFIN spec/validation; Plan, Compute and
// ApplyStructured are scripted.
type fakeFormat struct {
	core.Format
	// Waves: Plan returns the specs of the first wave not fully done (by
	// Kind+TargetPath among PlanState.Done with status done), nil when all done.
	Waves [][]core.TaskSpec
	// OnPlan observes every Plan call (what the format sees).
	OnPlan       func(st core.PlanState)
	ComputeFn    func(doc *core.Document) core.ComputeResult
	StructuredFn func(kind string, doc *core.Document, out json.RawMessage) error
	// InstructionsPath is the format instructions prompt (none when empty).
	InstructionsPath string
	DocQuestionsFn   func(doc *core.Document) []core.QuestionDraft
	CodeSemantics    map[string][]models.FieldSemanticType
	CodeFromValueFn  func(raw string) (string, bool)
}

func (f *fakeFormat) InstructionsPromptPath() string { return f.InstructionsPath }

// DocumentQuestions scripts core.DocumentQuestioner.
func (f *fakeFormat) DocumentQuestions(doc *core.Document) []core.QuestionDraft {
	if f.DocQuestionsFn == nil {
		return nil
	}
	return f.DocQuestionsFn(doc)
}

// CodeListSemantics and CodeFromValue script core.CodeSourceMapper.
func (f *fakeFormat) CodeListSemantics(list string) []models.FieldSemanticType {
	return f.CodeSemantics[list]
}

func (f *fakeFormat) CodeFromValue(_ string, _ models.FieldSemanticType, raw string) (string, bool) {
	if f.CodeFromValueFn == nil {
		return "", false
	}
	return f.CodeFromValueFn(raw)
}

func (f *fakeFormat) Plan(st core.PlanState) []core.TaskSpec {
	if f.OnPlan != nil {
		f.OnPlan(st)
	}
	done := map[string]bool{}
	for _, t := range st.Done {
		if t.Status == models.SarJobStatusDone {
			done[key(t.Kind, t.TargetPath)] = true
		}
	}
	for _, wave := range f.Waves {
		var pending []core.TaskSpec
		for _, s := range wave {
			if !done[key(s.Kind, string(s.TargetPath))] {
				pending = append(pending, s)
			}
		}
		if len(pending) > 0 {
			return wave // the orchestrator dedups by input_hash
		}
	}
	return nil
}

func (f *fakeFormat) Compute(doc *core.Document) core.ComputeResult {
	if f.ComputeFn == nil {
		return core.ComputeResult{}
	}
	return f.ComputeFn(doc)
}

func (f *fakeFormat) ApplyStructured(kind string, doc *core.Document, out json.RawMessage, _ core.StructuredContext) error {
	if f.StructuredFn == nil {
		return nil
	}
	return f.StructuredFn(kind, doc, out)
}

// Spec builds a TaskSpec; PromptPath is "prompts/<kind>.md".
func Spec(kind, target string, dependsOnKinds ...string) core.TaskSpec {
	return core.TaskSpec{
		Kind: kind, TargetPath: core.Path(target), PromptPath: "prompts/" + kind + ".md",
		Tier: core.TierLight, DependsOnKinds: dependsOnKinds,
		// Fill() cites t/o: the task inputs are the fallback allowed sources.
		Inputs: []core.ObjectRef{{Table: "t", ObjectID: "o"}},
	}
}

// fakePrompts: every path exists unless listed in Missing.
type fakePrompts struct{ Missing map[string]bool }

func (p fakePrompts) Exists(path string) bool { return !p.Missing[path] }

// ---------- in-memory repository ----------

// memRepo implements DeclarationRepository, SelectionRepository,
// GenerationRepository and DataModelReader. Claim/Complete/Fail follow the
// real SQL: Claim moves pending|failed|(running with started_at < staleBefore)
// to running, attempts+1, returns the attempt; Complete/Fail require
// status=running AND attempts=attempt, else (false, nil).
type memRepo struct {
	mu        sync.Mutex
	now       func() time.Time
	decl      models.SarDeclaration
	selection []models.SarSelectedObject
	jobs      map[uuid.UUID]*models.SarGenerationJob
	tasks     []*models.SarGenerationTask
	questions []models.SarQuestion
	dataModel models.DataModel
	// Touches counts heartbeats per job.
	Touches int
	// StatusHistory lists the successful job status transitions.
	StatusHistory []models.SarJobStatus
	// UpdateCalls counts document writes (to assert "version unchanged").
	UpdateCalls int
}

func newMemRepo(now func() time.Time) *memRepo {
	return &memRepo{now: now, jobs: map[uuid.UUID]*models.SarGenerationJob{}}
}

func (r *memRepo) GetSarDeclaration(context.Context, repositories.Executor, uuid.UUID) (models.SarDeclaration, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.decl, nil
}

// GetSarDeclarationForUpdate: FOR UPDATE is a no-op (the fake tx does not lock).
func (r *memRepo) GetSarDeclarationForUpdate(ctx context.Context, e repositories.Executor, id uuid.UUID) (models.SarDeclaration, error) {
	return r.GetSarDeclaration(ctx, e, id)
}

func (r *memRepo) UpdateSarDeclarationDocument(_ context.Context, _ repositories.Executor, _ uuid.UUID,
	expectedVersion int, document []byte, analysis, extraInfo string,
) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.decl.Version != expectedVersion {
		return 0, models.ErrSarVersionConflict
	}
	r.decl.Document, r.decl.Analysis, r.decl.ExtraInfo = document, analysis, extraInfo
	r.decl.Version++
	r.UpdateCalls++
	return r.decl.Version, nil
}

// ManualEdit simulates an analyst edit (version bump, origin manual).
func (r *memRepo) ManualEdit(t *testing.T, path core.Path, value string) {
	t.Helper()
	r.mu.Lock()
	defer r.mu.Unlock()
	doc := core.NewDocument()
	if len(r.decl.Document) > 0 {
		require.NoError(t, json.Unmarshal(r.decl.Document, doc))
	}
	doc.Values[path] = core.Value{Value: value, Origin: core.OriginManual}
	b, err := json.Marshal(doc)
	require.NoError(t, err)
	r.decl.Document = b
	r.decl.Version++
}

// Doc returns the current document.
func (r *memRepo) Doc(t *testing.T) *core.Document {
	t.Helper()
	r.mu.Lock()
	defer r.mu.Unlock()
	doc := core.NewDocument()
	if len(r.decl.Document) > 0 {
		require.NoError(t, json.Unmarshal(r.decl.Document, doc))
	}
	return doc
}

func (r *memRepo) ListSarSelection(context.Context, repositories.Executor, uuid.UUID) ([]models.SarSelectedObject, error) {
	return r.selection, nil
}

func (r *memRepo) GetDataModel(context.Context, uuid.UUID, models.DataModelReadOptions, bool) (models.DataModel, error) {
	return r.dataModel, nil
}

func (r *memRepo) GetSarGenerationJob(_ context.Context, _ repositories.Executor, id uuid.UUID) (models.SarGenerationJob, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	j, ok := r.jobs[id]
	if !ok {
		return models.SarGenerationJob{}, models.NotFoundError
	}
	return *j, nil
}

// SetSarGenerationJobStatus follows the SQL guard: pending|running ->
// running|done|failed only.
func (r *memRepo) SetSarGenerationJobStatus(ctx context.Context, _ repositories.Executor, id uuid.UUID,
	status models.SarJobStatus, errMsg string,
) (bool, error) {
	if err := ctx.Err(); err != nil { // like pgx on a cancelled context
		return false, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	j := r.jobs[id]
	if j.Status != models.SarJobStatusPending && j.Status != models.SarJobStatusRunning {
		return false, nil
	}
	j.Status, j.Error, j.UpdatedAt = status, errMsg, r.now()
	r.StatusHistory = append(r.StatusHistory, status)
	return true, nil
}

func (r *memRepo) SetSarGenerationJobAttempt(_ context.Context, _ repositories.Executor, id uuid.UUID, attempt int) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if j, ok := r.jobs[id]; ok { // like an UPDATE matching no row
		j.Attempt = attempt
	}
	return nil
}

func (r *memRepo) TouchSarGenerationJob(_ context.Context, _ repositories.Executor, id uuid.UUID) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.jobs[id].UpdatedAt = r.now()
	r.Touches++
	return nil
}

func (r *memRepo) MarkSarGenerationJobRateLimited(_ context.Context, _ repositories.Executor, id uuid.UUID) (time.Duration, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	j := r.jobs[id]
	if j.RateLimitedSince == nil {
		now := r.now()
		j.RateLimitedSince = &now
	}
	return r.now().Sub(*j.RateLimitedSince), nil
}

func (r *memRepo) ClearSarGenerationJobRateLimited(_ context.Context, _ repositories.Executor, id uuid.UUID) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.jobs[id].RateLimitedSince = nil
	return nil
}

func (r *memRepo) CreateSarGenerationTasks(_ context.Context, _ repositories.Executor, ts []models.SarGenerationTask) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, t := range ts {
		t := t
		dup := false
		for _, e := range r.tasks { // unique (job_id, input_hash), ON CONFLICT DO NOTHING
			dup = dup || (e.JobId == t.JobId && e.InputHash == t.InputHash)
		}
		if !dup {
			r.tasks = append(r.tasks, &t)
		}
	}
	return nil
}

func (r *memRepo) DeleteSarGenerationUnfinishedTasks(_ context.Context, _ repositories.Executor, jobId uuid.UUID, ids []uuid.UUID) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	kept := r.tasks[:0:0]
	for _, t := range r.tasks {
		del := t.JobId == jobId && t.Status != models.SarJobStatusDone && slices.Contains(ids, t.Id)
		if !del {
			kept = append(kept, t)
		}
	}
	r.tasks = kept
	return nil
}

func (r *memRepo) ListSarGenerationTasks(_ context.Context, _ repositories.Executor, jobId uuid.UUID) ([]models.SarGenerationTask, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []models.SarGenerationTask
	for _, t := range r.tasks {
		if t.JobId == jobId {
			c := *t
			c.Trace = nil // not loaded by the real listing
			out = append(out, c)
		}
	}
	return out, nil
}

func (r *memRepo) task(id uuid.UUID) *models.SarGenerationTask {
	for _, t := range r.tasks {
		if t.Id == id {
			return t
		}
	}
	return nil
}

func (r *memRepo) ClaimSarGenerationTask(_ context.Context, _ repositories.Executor, id uuid.UUID,
	staleAfter time.Duration,
) (int, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	t := r.task(id)
	if t == nil {
		return 0, false, nil
	}
	claimable := t.Status == models.SarJobStatusPending || t.Status == models.SarJobStatusFailed ||
		(t.Status == models.SarJobStatusRunning && t.StartedAt != nil && r.now().Sub(*t.StartedAt) > staleAfter)
	if !claimable {
		return 0, false, nil
	}
	now := r.now()
	t.Status, t.Attempts, t.StartedAt, t.Error = models.SarJobStatusRunning, t.Attempts+1, &now, ""
	return t.Attempts, true, nil
}

func (r *memRepo) CompleteSarGenerationTask(_ context.Context, _ repositories.Executor, id uuid.UUID,
	attempt int, output, trace []byte,
) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	t := r.task(id)
	if t == nil || t.Status != models.SarJobStatusRunning || t.Attempts != attempt {
		return false, nil
	}
	now := r.now()
	t.Status, t.Output, t.Trace, t.FinishedAt = models.SarJobStatusDone, output, trace, &now
	return true, nil
}

func (r *memRepo) FailSarGenerationTask(ctx context.Context, _ repositories.Executor, id uuid.UUID,
	attempt int, errMsg string, trace []byte,
) (bool, error) {
	if err := ctx.Err(); err != nil { // like pgx on a cancelled context
		return false, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	t := r.task(id)
	if t == nil || t.Status != models.SarJobStatusRunning || t.Attempts != attempt {
		return false, nil
	}
	now := r.now()
	t.Status, t.Error, t.Trace, t.FinishedAt = models.SarJobStatusFailed, errMsg, trace, &now
	return true, nil
}

func (r *memRepo) CreateSarQuestions(_ context.Context, _ repositories.Executor, qs []models.SarQuestion) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, q := range qs {
		if q.CreatedAt.IsZero() {
			q.CreatedAt = r.now()
		}
		r.questions = append(r.questions, q)
	}
	return nil
}

func (r *memRepo) ListSarQuestions(_ context.Context, _ repositories.Executor, declId uuid.UUID, status string) ([]models.SarQuestion, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []models.SarQuestion
	for _, q := range r.questions {
		if q.DeclarationId == declId && (status == "" || q.Status == status) {
			out = append(out, q)
		}
	}
	return out, nil
}

func (r *memRepo) CloseSarQuestions(_ context.Context, _ repositories.Executor, ids []uuid.UUID, status string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i := range r.questions {
		for _, id := range ids {
			if r.questions[i].Id == id {
				now := r.now()
				r.questions[i].Status, r.questions[i].ClosedAt = status, &now
			}
		}
	}
	return nil
}

// Task returns a copy of the task of the job with this kind and target.
func (r *memRepo) Task(t *testing.T, kind, target string) models.SarGenerationTask {
	t.Helper()
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, tk := range r.tasks {
		if tk.Kind == kind && tk.TargetPath == target {
			return *tk
		}
	}
	require.Failf(t, "task not found", "%s|%s", kind, target)
	return models.SarGenerationTask{}
}

// SetTask lets a test force a task state (e.g. a stale running task).
func (r *memRepo) SetTask(t *testing.T, kind, target string, mutate func(*models.SarGenerationTask)) {
	t.Helper()
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, tk := range r.tasks {
		if tk.Kind == kind && tk.TargetPath == target {
			mutate(tk)
			return
		}
	}
	require.Failf(t, "task not found", "%s|%s", kind, target)
}

type fakeExecFactory struct{}

func (fakeExecFactory) NewExecutor() repositories.Executor { return &mocks.Executor{} }

// fakeTxFactory runs fn on a placeholder transaction: no rollback, no lock.
type fakeTxFactory struct{}

func (fakeTxFactory) Transaction(_ context.Context, fn func(tx repositories.Transaction) error) error {
	return fn(&mocks.Transaction{})
}

// ---------- test harness ----------

// fakes groups what a test scripts and inspects.
type fakes struct {
	OrgId  uuid.UUID
	DeclId uuid.UUID
	JobId  uuid.UUID
	Repo   *memRepo
	Runner *fakeRunner
	Format *fakeFormat
	// Prompts: add paths to Missing to simulate a missing prompt.
	Prompts *fakePrompts
	// Clock is the fake clock (advance it with Advance).
	clockMu sync.Mutex
	clock   time.Time
}

func (f *fakes) Now() time.Time {
	f.clockMu.Lock()
	defer f.clockMu.Unlock()
	return f.clock
}

// Advance moves the fake clock forward (e.g. past StaleAfter).
func (f *fakes) Advance(d time.Duration) {
	f.clockMu.Lock()
	defer f.clockMu.Unlock()
	f.clock = f.clock.Add(d)
}

// newTestOrchestrator wires an Orchestrator on in-memory fakes with one
// declaration (TRACFIN 1.4.0, empty document, version 1) and one pending
// "generate" job (f.JobId). Parallelism 3, StaleAfter 1h.
func newTestOrchestrator(t *testing.T) (*Orchestrator, *fakes) {
	t.Helper()
	base, ok := formats.Get(core.FormatKey{Family: "tracfin", Country: "FR", Version: "1.4.0"})
	require.True(t, ok)
	f := &fakes{
		OrgId: uuid.New(), DeclId: uuid.New(), JobId: uuid.New(),
		Runner:  newFakeRunner(),
		Format:  &fakeFormat{Format: base},
		Prompts: &fakePrompts{Missing: map[string]bool{}},
		clock:   time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC),
	}
	f.Repo = newMemRepo(f.Now)
	doc, err := json.Marshal(core.NewDocument())
	require.NoError(t, err)
	f.Repo.decl = models.SarDeclaration{
		Id: f.DeclId, OrganizationId: f.OrgId, CaseId: uuid.New(), FormatKey: testFormatKey,
		ReportType: "DS_INITIALE", Document: doc, Version: 1,
	}
	f.Repo.jobs[f.JobId] = &models.SarGenerationJob{
		Id: f.JobId, DeclarationId: f.DeclId, Kind: models.SarJobKindGenerate,
		Status: models.SarJobStatusPending, UpdatedAt: f.Now(),
	}
	o := NewOrchestrator(Deps{
		ExecutorFactory: fakeExecFactory{}, TransactionFactory: fakeTxFactory{},
		Declarations: f.Repo, Selection: f.Repo, Generation: f.Repo, DataModels: f.Repo,
		Runner: f.Runner, Prompts: f.Prompts,
		FormatGetter: func(core.FormatKey) (core.Format, bool) { return f.Format, true },
		Parallelism:  3, StaleAfter: time.Hour,
	})
	return o, f
}

// Fill builds an LLM-looking fill on a path (the orchestrator restamps Origin
// and TaskID anyway).
func Fill(path, value string) core.Fill {
	return core.Fill{
		Path: core.Path(path), Value: value, Origin: core.OriginLLM, Confidence: core.ConfidenceHigh,
		Justification: "test", Sources: []core.SourceRef{{Object: core.ObjectRef{Table: "t", ObjectID: "o"}, Field: "f", RawValue: "v"}},
	}
}

var errProvider = errors.New("provider unavailable")

// RateLimited is a runner error the worker turns into a snooze.
func RateLimited() error { return errors.Wrap(models.LLMRateLimitedError, "429") }
