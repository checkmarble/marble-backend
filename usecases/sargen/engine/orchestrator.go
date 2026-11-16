// Package engine runs persisted SAR generation jobs: it plans tasks with the
// format, runs them through a TaskRunner and applies their output to the
// declaration, resumably.
package engine

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io/fs"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/google/uuid"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/sargen/core"
	"github.com/checkmarble/marble-backend/utils"
)

const (
	DefaultParallelism = 3
	// DefaultTimeout is the default SAR_GENERATION_TIMEOUT.
	DefaultTimeout = 30 * time.Minute
	// RateLimitSnooze is how long the worker snoozes on models.LLMRateLimitedError.
	RateLimitSnooze = 30 * time.Second
)

// RunningElsewhereSnooze is how long the worker snoozes when the only
// unfinished tasks are running under another worker (no attempt consumed).
const RunningElsewhereSnooze = 2 * time.Minute

// finalWriteTimeout bounds the writes made after the job context is done
// (task failure, final job status): they must survive a cancelled context.
const finalWriteTimeout = 10 * time.Second

var (
	// ErrTasksRunningElsewhere: nothing is claimable because the unfinished
	// tasks run under another worker (or a stale one not yet reclaimable).
	ErrTasksRunningElsewhere = errors.New("des tâches sont en cours d'exécution ailleurs")
	// ErrPromptMissing: a prompt file is missing; the job is marked failed and
	// retrying is pointless (the worker cancels the River job).
	ErrPromptMissing = errors.New("prompt manquant")
)

// detached returns a context that survives the cancellation of ctx.
func detached(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.WithoutCancel(ctx), finalWriteTimeout)
}

// TaskRunner runs one task (LLM agent loop in Task 13; a fake in tests). The
// returned error is technical (provider down, timeout, network): the task is
// marked failed and River retries the job. It must wrap
// models.LLMRateLimitedError on a rate limit.
type TaskRunner interface {
	Run(ctx context.Context, tc core.ToolContext, task models.SarGenerationTask, spec core.TaskSpec) (core.TaskOutput, error)
}

// PromptSource tells whether a prompt file exists in the prompts FS.
type PromptSource interface {
	Exists(path string) bool
}

// DeclarationRepository is the subset of the declaration repository used here.
type DeclarationRepository interface {
	GetSarDeclaration(ctx context.Context, exec repositories.Executor, id uuid.UUID) (models.SarDeclaration, error)
	// GetSarDeclarationForUpdate locks the row (FOR UPDATE) until commit.
	GetSarDeclarationForUpdate(ctx context.Context, exec repositories.Executor, id uuid.UUID) (models.SarDeclaration, error)
	// UpdateSarDeclarationDocument returns the new version, or models.ErrSarVersionConflict.
	UpdateSarDeclarationDocument(ctx context.Context, exec repositories.Executor, id uuid.UUID,
		expectedVersion int, document []byte, analysis, extraInfo string) (int, error)
}

type SelectionRepository interface {
	ListSarSelection(ctx context.Context, exec repositories.Executor, declarationId uuid.UUID) ([]models.SarSelectedObject, error)
}

// GenerationRepository is the subset of the jobs/tasks/questions repository.
// Claim returns the attempt number the caller owns; Complete and Fail only
// succeed (true) for a task still running with that attempt, otherwise the
// caller was superseded and must discard its result without applying it.
type GenerationRepository interface {
	GetSarGenerationJob(ctx context.Context, exec repositories.Executor, id uuid.UUID) (models.SarGenerationJob, error)
	SetSarGenerationJobStatus(ctx context.Context, exec repositories.Executor, id uuid.UUID,
		status models.SarJobStatus, errMsg string) error
	TouchSarGenerationJob(ctx context.Context, exec repositories.Executor, id uuid.UUID) error
	CreateSarGenerationTasks(ctx context.Context, exec repositories.Executor, tasks []models.SarGenerationTask) error
	ListSarGenerationTasks(ctx context.Context, exec repositories.Executor, jobId uuid.UUID) ([]models.SarGenerationTask, error)
	ClaimSarGenerationTask(ctx context.Context, exec repositories.Executor, id uuid.UUID,
		staleBefore time.Time) (attempt int, ok bool, err error)
	CompleteSarGenerationTask(ctx context.Context, exec repositories.Executor, id uuid.UUID,
		attempt int, output, trace []byte) (bool, error)
	FailSarGenerationTask(ctx context.Context, exec repositories.Executor, id uuid.UUID,
		attempt int, errMsg string, trace []byte) (bool, error)
	CreateSarQuestions(ctx context.Context, exec repositories.Executor, qs []models.SarQuestion) error
	ListSarQuestions(ctx context.Context, exec repositories.Executor, declarationId uuid.UUID, status string) ([]models.SarQuestion, error)
	CloseSarQuestions(ctx context.Context, exec repositories.Executor, ids []uuid.UUID, status string) error
}

type DataModelReader interface {
	GetDataModel(ctx context.Context, organizationID uuid.UUID, options models.DataModelReadOptions,
		useCache bool) (models.DataModel, error)
}

type ExecutorFactory interface {
	NewExecutor() repositories.Executor
}

type TransactionFactory interface {
	Transaction(ctx context.Context, fn func(tx repositories.Transaction) error) error
}

// Deps groups the collaborators of an Orchestrator.
type Deps struct {
	ExecutorFactory    ExecutorFactory
	TransactionFactory TransactionFactory
	Declarations       DeclarationRepository
	Selection          SelectionRepository
	Generation         GenerationRepository
	DataModels         DataModelReader
	Runner             TaskRunner
	Prompts            PromptSource
	// FormatGetter resolves a format key; defaults to formats.Get in the
	// constructor wiring (usecases_with_creds.go), injected in tests.
	FormatGetter func(core.FormatKey) (core.Format, bool)
	// Parallelism is SAR_GENERATION_PARALLELISM (<=0 -> DefaultParallelism).
	Parallelism int
	// StaleAfter: a running task started longer ago is presumed dead and can
	// be claimed again (2 x SAR_GENERATION_TIMEOUT in production).
	StaleAfter time.Duration
	// Now is the clock (nil -> time.Now).
	Now func() time.Time
}

// Orchestrator runs generation jobs. All reads use the declaration's
// organization: the worker context carries no user credentials.
type Orchestrator struct {
	executorFactory    ExecutorFactory
	transactionFactory TransactionFactory
	declarations       DeclarationRepository
	selection          SelectionRepository
	generation         GenerationRepository
	dataModels         DataModelReader
	runner             TaskRunner
	prompts            PromptSource
	formatGetter       func(core.FormatKey) (core.Format, bool)
	parallelism        int
	staleAfter         time.Duration
	now                func() time.Time
}

func NewOrchestrator(d Deps) *Orchestrator {
	o := &Orchestrator{
		executorFactory:    d.ExecutorFactory,
		transactionFactory: d.TransactionFactory,
		declarations:       d.Declarations,
		selection:          d.Selection,
		generation:         d.Generation,
		dataModels:         d.DataModels,
		runner:             d.Runner,
		prompts:            d.Prompts,
		formatGetter:       d.FormatGetter,
		parallelism:        d.Parallelism,
		staleAfter:         d.StaleAfter,
		now:                d.Now,
	}
	if o.parallelism <= 0 {
		o.parallelism = DefaultParallelism
	}
	if o.staleAfter <= 0 {
		o.staleAfter = 2 * DefaultTimeout
	}
	if o.now == nil {
		o.now = time.Now
	}
	return o
}

// RunJob runs (or resumes) a job. Loop: Format.Plan (a spec whose input_hash
// already exists in the job is not recreated), run the ready tasks (dependencies
// done) with at most Parallelism in parallel; each task: Claim -> Run -> one
// transaction: re-read the declaration FOR UPDATE, ApplyStructured if
// Structured, ApplyFills (AllowedPrefixes = AllowedPaths or [TargetPath],
// OverwriteNonManual, Origin=OriginLLM and TaskID stamped by the orchestrator
// on every fill, AllowedSources nil until Task 13), rewrite QuestionDraft
// paths and option fills with ApplyResult.Resolved/Rewrite, persist questions
// without duplicates (same paths and text, open or closed), save the document
// with the version read, CompleteSarGenerationTask(attempt); conflicts and
// rejections stay in the task output. Compute (Origin=OriginComputed,
// OverwriteNonManual, SkipMaxItems, RemoveItems first) only when no fill task
// remains. Review tasks write nothing and replace the open review_finding
// questions with their Findings. If Complete/Fail report false (superseded),
// the result is discarded and nothing is applied. Before launching the tasks
// of a step, every PromptPath must exist in Prompts, else the job is set
// failed with "prompt manquant : <chemin>". A technical task error ->
// FailSarGenerationTask, other running tasks finish, then RunJob returns the
// error (River retries); a models.LLMRateLimitedError must stay detectable
// with errors.Is. Heartbeat: TouchSarGenerationJob between tasks. On success
// the job is set done.
func (o *Orchestrator) RunJob(ctx context.Context, jobId uuid.UUID) error {
	exec := o.executorFactory.NewExecutor()
	job, err := o.generation.GetSarGenerationJob(ctx, exec, jobId)
	if err != nil {
		return errors.Wrap(err, "cannot read generation job")
	}
	if job.Status == models.SarJobStatusDone || job.Status == models.SarJobStatusFailed {
		return nil
	}
	decl, err := o.declarations.GetSarDeclaration(ctx, exec, job.DeclarationId)
	if err != nil {
		return errors.Wrap(err, "cannot read declaration")
	}
	key, err := core.ParseFormatKey(decl.FormatKey)
	if err != nil {
		return err
	}
	format, ok := o.formatGetter(key)
	if !ok {
		return errors.Newf("format %s introuvable", decl.FormatKey)
	}
	selection, err := o.selection.ListSarSelection(ctx, exec, decl.Id)
	if err != nil {
		return errors.Wrap(err, "cannot read selection")
	}
	// The declaration's organization, never one from user credentials.
	dataModel, err := o.dataModels.GetDataModel(ctx, decl.OrganizationId, models.DataModelReadOptions{}, true)
	if err != nil {
		return errors.Wrap(err, "cannot read data model")
	}
	run := &jobRun{
		o: o, job: job, declId: decl.Id, orgId: decl.OrganizationId, format: format,
		selection: selection, dataModel: dataModel,
	}
	return run.loop(ctx)
}

// jobRun is the state of one RunJob call.
type jobRun struct {
	o         *Orchestrator
	job       models.SarGenerationJob
	declId    uuid.UUID
	orgId     uuid.UUID
	format    core.Format
	selection []models.SarSelectedObject
	dataModel models.DataModel
	// applyMu serializes document writes inside this process; across
	// processes the FOR UPDATE lock does.
	applyMu sync.Mutex
}

func (r *jobRun) loop(ctx context.Context) error {
	o := r.o
	exec := o.executorFactory.NewExecutor()
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		tasks, err := o.generation.ListSarGenerationTasks(ctx, exec, r.job.Id)
		if err != nil {
			return errors.Wrap(err, "cannot list tasks")
		}
		decl, err := o.declarations.GetSarDeclaration(ctx, exec, r.declId)
		if err != nil {
			return errors.Wrap(err, "cannot read declaration")
		}
		doc, err := decodeDocument(decl.Document)
		if err != nil {
			return err
		}
		var done []models.SarGenerationTask
		for _, t := range tasks {
			if t.Status == models.SarJobStatusDone {
				done = append(done, t)
			}
		}
		specs := r.format.Plan(core.PlanState{
			JobKind: string(r.job.Kind), Declaration: decl, Doc: doc,
			Selection: r.selection, Done: done, Issues: r.format.Validate(doc),
		})

		// Create the planned tasks that do not exist yet (by input_hash).
		known := map[string]bool{}
		for _, t := range tasks {
			known[t.InputHash] = true
		}
		var fresh []models.SarGenerationTask
		for _, s := range specs {
			t := taskFromSpec(r.job.Id, s, r.selection)
			if known[t.InputHash] {
				continue
			}
			if err := r.checkPrompt(ctx, s); err != nil {
				return err
			}
			known[t.InputHash] = true
			fresh = append(fresh, t)
		}
		if len(fresh) > 0 {
			if err := o.generation.CreateSarGenerationTasks(ctx, exec, fresh); err != nil {
				return errors.Wrap(err, "cannot create tasks")
			}
			tasks = append(tasks, fresh...)
		}

		ready, pending, err := r.readyTasks(tasks)
		if err != nil {
			return err
		}
		// Resumed tasks too: a prompt may have disappeared since the plan.
		for _, rt := range ready {
			if err := r.checkPrompt(ctx, rt.spec); err != nil {
				return err
			}
		}
		if len(ready) == 0 {
			if pending > 0 {
				return errors.New("des tâches ne peuvent pas démarrer (dépendances non terminées)")
			}
			// Nothing left to launch.
			if len(fresh) > 0 {
				continue
			}
			return r.finish(ctx)
		}

		progress, err := r.runWave(ctx, ready)
		if err != nil {
			return err
		}
		if !progress {
			return ErrTasksRunningElsewhere
		}
	}
}

// checkPrompt fails the job when the prompt of a spec does not exist.
func (r *jobRun) checkPrompt(ctx context.Context, s core.TaskSpec) error {
	if s.PromptPath == "" || r.o.prompts.Exists(s.PromptPath) {
		return nil
	}
	msg := "prompt manquant : " + s.PromptPath
	dctx, cancel := detached(ctx)
	defer cancel()
	if err := r.o.generation.SetSarGenerationJobStatus(dctx, r.o.executorFactory.NewExecutor(),
		r.job.Id, models.SarJobStatusFailed, msg); err != nil {
		return errors.Wrap(err, "cannot fail job")
	}
	return errors.Mark(errors.New(msg), ErrPromptMissing)
}

// readyTasks returns the not-done tasks whose dependencies (their spec's
// DependsOnKinds, among the tasks of the job) are done, and how many not-done
// tasks are blocked.
func (r *jobRun) readyTasks(tasks []models.SarGenerationTask) (ready []readyTask, blocked int, err error) {
	notDone := map[string]bool{}
	for _, t := range tasks {
		if t.Status != models.SarJobStatusDone {
			notDone[t.Kind] = true
		}
	}
	for _, t := range tasks {
		if t.Status == models.SarJobStatusDone {
			continue
		}
		var in taskInput
		if err := json.Unmarshal(t.Input, &in); err != nil {
			return nil, 0, errors.Wrapf(err, "invalid input of task %s", t.Id)
		}
		wait := false
		for _, k := range in.Spec.DependsOnKinds {
			if notDone[k] {
				wait = true
				break
			}
		}
		if wait {
			blocked++
			continue
		}
		ready = append(ready, readyTask{task: t, spec: in.Spec})
	}
	return ready, blocked, nil
}

type readyTask struct {
	task models.SarGenerationTask
	spec core.TaskSpec
}

// runWave runs the ready tasks (bounded parallelism). It returns whether any
// task was claimed; the first technical error (rate limit preferred) is
// returned once every in-flight task has finished.
func (r *jobRun) runWave(ctx context.Context, ready []readyTask) (bool, error) {
	var (
		wg       sync.WaitGroup
		mu       sync.Mutex
		claimed  bool
		firstErr error
		rateErr  error
		sem      = make(chan struct{}, r.o.parallelism)
		failed   bool
	)
	for _, rt := range ready {
		mu.Lock()
		stop := failed
		mu.Unlock()
		if stop {
			break // do not start new tasks after a failure
		}
		sem <- struct{}{}
		mu.Lock()
		stop = failed
		mu.Unlock()
		if stop { // a task failed while waiting for a slot
			<-sem
			break
		}
		wg.Add(1)
		go func(rt readyTask) {
			defer wg.Done()
			defer func() { <-sem }()
			ok, err := r.runTask(ctx, rt)
			mu.Lock()
			defer mu.Unlock()
			claimed = claimed || ok
			if err != nil {
				failed = true
				if errors.Is(err, models.LLMRateLimitedError) && rateErr == nil {
					rateErr = err
				}
				if firstErr == nil {
					firstErr = err
				}
			}
		}(rt)
	}
	wg.Wait()
	if rateErr != nil {
		return claimed, rateErr
	}
	return claimed, firstErr
}

// runTask claims, runs and applies one task. claimed is false when the task
// was not claimable (done, or running elsewhere). A non-nil error is the
// technical error of the task, already recorded with FailSarGenerationTask.
func (r *jobRun) runTask(ctx context.Context, rt readyTask) (claimed bool, err error) {
	o := r.o
	exec := o.executorFactory.NewExecutor()
	attempt, ok, err := o.generation.ClaimSarGenerationTask(ctx, exec, rt.task.Id, o.now().Add(-o.staleAfter))
	if err != nil {
		return false, errors.Wrap(err, "cannot claim task")
	}
	if !ok {
		return false, nil
	}
	fail := func(cause error, trace []byte) error {
		// Detached: on a timeout or shutdown ctx is done, and the task must
		// still end up failed (claimable) rather than stuck running.
		dctx, cancel := detached(ctx)
		defer cancel()
		if _, ferr := o.generation.FailSarGenerationTask(dctx, exec, rt.task.Id, attempt, cause.Error(), trace); ferr != nil {
			return errors.Wrap(cause, "cannot record task failure: "+ferr.Error())
		}
		return errors.Wrapf(cause, "task %s %s", rt.task.Kind, rt.task.TargetPath)
	}

	defer func() {
		if rec := recover(); rec != nil {
			err = fail(errors.Newf("panic dans la tâche : %v", rec), nil)
		}
	}()

	decl, err := o.declarations.GetSarDeclaration(ctx, exec, r.declId)
	if err != nil {
		return true, fail(err, nil)
	}
	doc, err := decodeDocument(decl.Document)
	if err != nil {
		return true, fail(err, nil)
	}
	tc := core.ToolContext{
		OrgId: r.orgId, DeclarationId: r.declId, TaskId: rt.task.Id, Format: r.format,
		Doc: doc.Clone(), Selection: r.selection, DataModel: r.dataModel,
	}
	task := rt.task
	task.Attempts = attempt
	out, runErr := o.runner.Run(ctx, tc, task, rt.spec)
	if runErr != nil {
		return true, fail(runErr, out.Trace)
	}

	superseded := false
	applyErr := func() error {
		r.applyMu.Lock()
		defer r.applyMu.Unlock()
		return o.transactionFactory.Transaction(ctx, func(tx repositories.Transaction) error {
			var err error
			superseded, err = r.applyOutput(ctx, tx, rt, attempt, out)
			return err
		})
	}()
	if applyErr != nil {
		return true, fail(applyErr, out.Trace)
	}
	if superseded {
		return true, nil
	}
	// Heartbeat: a live job is told apart from a dead one by updated_at.
	// The output is applied: a failed heartbeat must not fail the task.
	if err := o.generation.TouchSarGenerationJob(ctx, exec, r.job.Id); err != nil {
		utils.LoggerFromContext(ctx).WarnContext(ctx, "sar generation: heartbeat failed",
			"job_id", r.job.Id, "error", err.Error())
	}
	return true, nil
}

// taskResult is the persisted output of a task.
type taskResult struct {
	Applied   []core.Path    `json:"applied"`
	Rejected  []rejectedView `json:"rejected"`
	Conflicts []conflictView `json:"conflicts"`
	Questions int            `json:"questions"`
	Findings  int            `json:"findings"`
}

type rejectedView struct {
	Path    core.Path `json:"path"`
	Code    string    `json:"code"`
	Message string    `json:"message"`
}

// conflictView keeps a refused fill: Value is what the task proposed, Current
// what stayed in the document.
type conflictView struct {
	Path    core.Path  `json:"path"`
	Current core.Value `json:"current"`
	Value   string     `json:"value"`
	Fill    core.Fill  `json:"fill"`
}

// applyOutput applies a task output and completes the task, in tx, with the
// declaration read FOR UPDATE. Everything is computed in memory first and
// Complete is called before anything is persisted: when it reports false
// (superseded attempt) nothing is written and superseded is true.
func (r *jobRun) applyOutput(ctx context.Context, tx repositories.Transaction, rt readyTask, attempt int,
	out core.TaskOutput,
) (superseded bool, err error) {
	o := r.o
	decl, err := o.declarations.GetSarDeclarationForUpdate(ctx, tx, r.declId)
	if err != nil {
		return false, errors.Wrap(err, "cannot lock declaration")
	}
	doc, err := decodeDocument(decl.Document)
	if err != nil {
		return false, err
	}
	taskId := rt.task.Id
	isReview := rt.spec.Kind == core.TaskKindReview
	var (
		res     core.ApplyResult
		changed bool
		result  taskResult
		qs      []models.SarQuestion
		closeQs []uuid.UUID
	)

	if !isReview {
		if len(out.Structured) > 0 {
			if err := r.format.ApplyStructured(rt.spec.Kind, doc, out.Structured); err != nil {
				return false, errors.Wrap(err, "invalid structured output")
			}
			changed = true
		}
		fills := make([]core.Fill, len(out.Fills))
		for i, f := range out.Fills {
			f.Origin, f.TaskID = core.OriginLLM, &taskId
			fills[i] = f
		}
		allowed := rt.spec.AllowedPaths
		if len(allowed) == 0 {
			allowed = []core.Path{rt.spec.TargetPath}
		}
		res = core.ApplyFills(r.format.Spec(), doc, fills, core.ApplyOptions{
			AllowedPrefixes: allowed, Overwrite: core.OverwriteNonManual,
		})
		changed = changed || len(res.Applied) > 0
		result.Applied = res.Applied
		for _, rj := range res.Rejected {
			result.Rejected = append(result.Rejected, rejectedView{
				Path: rj.Fill.Path, Code: rj.Issue.Code, Message: rj.Issue.Message,
			})
		}
		for _, c := range res.Conflicts {
			result.Conflicts = append(result.Conflicts, conflictView{
				Path: c.Path, Current: c.Before, Value: c.After.Value, Fill: c.After,
			})
		}

		existing, err := o.generation.ListSarQuestions(ctx, tx, r.declId, "")
		if err != nil {
			return false, errors.Wrap(err, "cannot list questions")
		}
		seen := map[string]bool{}
		for _, q := range existing {
			if q.Kind == models.SarQuestionKindQuestion {
				seen[questionKey(q.Paths, q.Text)] = true
			}
		}
		for _, d := range out.Questions {
			paths := make([]string, len(d.Paths))
			for i, p := range d.Paths {
				paths[i] = string(res.Rewrite(p))
			}
			k := questionKey(paths, d.Text)
			if seen[k] {
				continue
			}
			seen[k] = true
			opts := make([]core.QuestionOption, len(d.Options))
			for i, op := range d.Options {
				op.Fills = append([]core.Fill(nil), op.Fills...)
				for j := range op.Fills {
					op.Fills[j].Path = res.Rewrite(op.Fills[j].Path)
				}
				opts[i] = op
			}
			var rawOpts []byte
			if len(opts) > 0 {
				if rawOpts, err = json.Marshal(opts); err != nil {
					return false, errors.Wrap(err, "cannot encode question options")
				}
			}
			qs = append(qs, models.SarQuestion{
				Id: uuid.New(), DeclarationId: r.declId, TaskId: &taskId,
				Kind: models.SarQuestionKindQuestion, Paths: paths, Text: d.Text,
				Options: rawOpts, Status: models.SarQuestionStatusOpen,
			})
		}
		result.Questions = len(qs)
	} else {
		open, err := o.generation.ListSarQuestions(ctx, tx, r.declId, models.SarQuestionStatusOpen)
		if err != nil {
			return false, errors.Wrap(err, "cannot list questions")
		}
		for _, q := range open {
			if q.Kind == models.SarQuestionKindReviewFinding {
				closeQs = append(closeQs, q.Id)
			}
		}
		for _, fd := range out.Findings {
			paths := make([]string, len(fd.Paths))
			for i, p := range fd.Paths {
				paths[i] = string(p)
			}
			qs = append(qs, models.SarQuestion{
				Id: uuid.New(), DeclarationId: r.declId, TaskId: &taskId,
				Kind: models.SarQuestionKindReviewFinding, Severity: fd.Severity,
				Paths: paths, Text: fd.Message, Status: models.SarQuestionStatusOpen,
			})
		}
		result.Findings = len(qs)
	}

	rawOut, err := json.Marshal(result)
	if err != nil {
		return false, errors.Wrap(err, "cannot encode task output")
	}
	ok, err := o.generation.CompleteSarGenerationTask(ctx, tx, taskId, attempt, rawOut, out.Trace)
	if err != nil {
		return false, errors.Wrap(err, "cannot complete task")
	}
	if !ok {
		return true, nil // superseded: nothing persisted
	}
	if changed {
		if err := r.saveDocument(ctx, tx, decl, doc); err != nil {
			return false, err
		}
	}
	if err := o.generation.CloseSarQuestions(ctx, tx, closeQs, models.SarQuestionStatusDismissed); err != nil {
		return false, errors.Wrap(err, "cannot close findings")
	}
	if err := o.generation.CreateSarQuestions(ctx, tx, qs); err != nil {
		return false, errors.Wrap(err, "cannot create questions")
	}
	return false, nil
}

func (r *jobRun) saveDocument(ctx context.Context, exec repositories.Executor, decl models.SarDeclaration, doc *core.Document) error {
	raw, err := json.Marshal(doc)
	if err != nil {
		return errors.Wrap(err, "cannot encode document")
	}
	if _, err := r.o.declarations.UpdateSarDeclarationDocument(ctx, exec, decl.Id, decl.Version, raw,
		decl.Analysis, decl.ExtraInfo); err != nil {
		return errors.Wrap(err, "cannot save document")
	}
	return nil
}

// finish runs Compute (generation jobs only; every fill task is done) and
// marks the job done.
func (r *jobRun) finish(ctx context.Context) error {
	o := r.o
	if r.job.Kind != models.SarJobKindReview {
		err := func() error {
			r.applyMu.Lock()
			defer r.applyMu.Unlock()
			return o.transactionFactory.Transaction(ctx, func(tx repositories.Transaction) error {
				decl, err := o.declarations.GetSarDeclarationForUpdate(ctx, tx, r.declId)
				if err != nil {
					return errors.Wrap(err, "cannot lock declaration")
				}
				doc, err := decodeDocument(decl.Document)
				if err != nil {
					return err
				}
				c := r.format.Compute(doc)
				for _, p := range c.RemoveItems {
					doc.RemoveItem(p)
				}
				fills := make([]core.Fill, len(c.Fills))
				for i, f := range c.Fills {
					f.Origin, f.TaskID = core.OriginComputed, nil
					fills[i] = f
				}
				res := core.ApplyFills(r.format.Spec(), doc, fills, core.ApplyOptions{
					Overwrite: core.OverwriteNonManual, SkipMaxItems: true,
				})
				if len(res.Rejected) > 0 || len(res.Conflicts) > 0 {
					utils.LoggerFromContext(ctx).WarnContext(ctx, "sar generation: computed fills not applied",
						"job_id", r.job.Id, "rejected", len(res.Rejected), "conflicts", len(res.Conflicts),
						"first_rejected", firstRejected(res))
				}
				if len(c.RemoveItems) == 0 && len(res.Applied) == 0 {
					return nil
				}
				return r.saveDocument(ctx, tx, decl, doc)
			})
		}()
		if err != nil {
			return errors.Wrap(err, "compute failed")
		}
	}
	exec := o.executorFactory.NewExecutor()
	dctx, cancel := detached(ctx)
	defer cancel()
	if err := o.generation.SetSarGenerationJobStatus(dctx, exec, r.job.Id, models.SarJobStatusDone, ""); err != nil {
		return errors.Wrap(err, "cannot complete job")
	}
	return nil
}

func decodeDocument(raw []byte) (*core.Document, error) {
	doc := core.NewDocument()
	if len(raw) == 0 {
		return doc, nil
	}
	if err := json.Unmarshal(raw, doc); err != nil {
		return nil, errors.Wrap(err, "invalid declaration document")
	}
	return doc, nil
}

func firstRejected(res core.ApplyResult) string {
	if len(res.Rejected) > 0 {
		return string(res.Rejected[0].Fill.Path) + ": " + res.Rejected[0].Issue.Message
	}
	return ""
}

// questionKey identifies a question by its (order-independent) paths and text.
func questionKey(paths []string, text string) string {
	sorted := append([]string(nil), paths...)
	slices.Sort(sorted)
	return strings.Join(sorted, "\x00") + "\x01" + text
}

// taskInput is the persisted Input of a task: enough to rebuild its TaskSpec
// on resume, plus the frozen copies of its input objects.
type taskInput struct {
	Spec      core.TaskSpec    `json:"spec"`
	Snapshots []map[string]any `json:"snapshots"`
}

// hashedInput is what input_hash covers.
type hashedInput struct {
	Kind         string           `json:"kind"`
	TargetPath   core.Path        `json:"target_path"`
	AllowedPaths []core.Path      `json:"allowed_paths"`
	Inputs       []core.ObjectRef `json:"inputs"`
	Payload      map[string]any   `json:"payload"`
	Snapshots    []map[string]any `json:"snapshots"`
}

// taskFromSpec builds the pending task for a spec: input_hash is the sha256 of
// the sorted JSON (Go sorts map keys) of the kind, target, allowed paths,
// inputs, payload and the frozen snapshots of the inputs found in sel.
// DependsOn is left empty: dependencies (DependsOnKinds) are resolved at run
// time from the spec.
func taskFromSpec(jobId uuid.UUID, s core.TaskSpec, sel []models.SarSelectedObject) models.SarGenerationTask {
	snaps := make([]map[string]any, len(s.Inputs))
	for i, ref := range s.Inputs {
		for _, o := range sel {
			if o.TableName == ref.Table && o.ObjectId == ref.ObjectID {
				snaps[i] = o.Snapshot
				break
			}
		}
	}
	hb, _ := json.Marshal(hashedInput{
		Kind: s.Kind, TargetPath: s.TargetPath, AllowedPaths: s.AllowedPaths,
		Inputs: s.Inputs, Payload: s.Payload, Snapshots: snaps,
	})
	sum := sha256.Sum256(hb)
	in, _ := json.Marshal(taskInput{Spec: s, Snapshots: snaps})
	return models.SarGenerationTask{
		Id:         uuid.New(),
		JobId:      jobId,
		Kind:       s.Kind,
		TargetPath: string(s.TargetPath),
		Input:      in,
		InputHash:  hex.EncodeToString(sum[:]),
		DependsOn:  []uuid.UUID{},
		Status:     models.SarJobStatusPending,
	}
}

// FSPrompts is a PromptSource backed by the prompts filesystem.
type FSPrompts struct{ FS fs.FS }

func (p FSPrompts) Exists(path string) bool {
	if p.FS == nil {
		return false
	}
	_, err := fs.Stat(p.FS, path)
	return err == nil
}

// UnavailableRunner is the TaskRunner wired until the agent lands (Task 13).
type UnavailableRunner struct{}

func (UnavailableRunner) Run(context.Context, core.ToolContext, models.SarGenerationTask, core.TaskSpec) (core.TaskOutput, error) {
	return core.TaskOutput{}, errors.New("agent de génération non disponible")
}
