// Package engine runs persisted SAR generation jobs: it plans tasks with the
// format, runs them through a TaskRunner and applies their output to the
// declaration, resumably.
package engine

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io/fs"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
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
	// RateLimitMaxDuration is how long a job may keep hitting the provider's
	// rate limit before it fails.
	RateLimitMaxDuration = 2 * time.Hour
)

// RunningElsewhereSnooze is how long the worker snoozes when the only
// unfinished tasks are running under another worker (no attempt consumed).
const RunningElsewhereSnooze = 2 * time.Minute

// finalWriteTimeout bounds the writes made after the job context is done
// (task failure, final job status): they must survive a cancelled context.
const finalWriteTimeout = 10 * time.Second

// The failure message of a task that hit a limit: the reason comes before the
// advice (a relaunch without a change meets the same limit: the budget only
// depends on the size of the selection).
const (
	limitMessagePrefix = "une étape essentielle de la génération n'a pas abouti (limite atteinte"
	limitMessageAdvice = ". Réduisez la sélection ou relancez la génération."
)

// limitReachedMessage is the analyst-facing message of ErrTaskLimitReached for
// a limit reason (French, may be empty).
func limitReachedMessage(reason string) string {
	if reason == "" {
		return ErrTaskLimitReached.Error()
	}
	return limitMessagePrefix + " : " + reason + ")" + limitMessageAdvice
}

var (
	// ErrTasksRunningElsewhere: nothing is claimable because the unfinished
	// tasks run under another worker (or a stale one not yet reclaimable).
	ErrTasksRunningElsewhere = errors.New("des tâches sont en cours d'exécution ailleurs")
	// ErrPromptMissing: a prompt file is missing; the job is marked failed and
	// retrying is pointless (the worker cancels the River job).
	ErrPromptMissing = errors.New("prompt manquant")

	// ErrTaskLimitReached: a task flagged FailJobOnLimit ended on a limit with
	// no usable output; the job is marked failed (resumable) and the River job
	// cancelled, as a retry would meet the same limit.
	ErrTaskLimitReached = errors.New(limitMessagePrefix + ")" + limitMessageAdvice)
	// ErrJobGone: the job or its declaration no longer exists; retrying is
	// pointless (the worker cancels the River job).
	ErrJobGone = errors.New("génération ou déclaration introuvable")
	// ErrJobNotRunning: the job was failed or finished while a task was
	// running; its output was discarded, the task released and this run must
	// stop (the worker cancels the River job; a resume starts a new run).
	ErrJobNotRunning = errors.New("génération arrêtée")
	// ErrRateLimitedTooLong: the provider rate-limited the job for more than
	// RateLimitMaxDuration; the job is marked failed and the River job cancelled.
	ErrRateLimitedTooLong = errors.New("limite de débit du fournisseur LLM dépassée trop longtemps")
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
	// SetSarGenerationJobStatus is a guarded transition (pending|running ->
	// running|done|failed); false means the job was already finished.
	SetSarGenerationJobStatus(ctx context.Context, exec repositories.Executor, id uuid.UUID,
		status models.SarJobStatus, errMsg string) (bool, error)
	TouchSarGenerationJob(ctx context.Context, exec repositories.Executor, id uuid.UUID) error
	// MarkSarGenerationJobRateLimited records a provider rate limit and returns
	// how long the current streak lasts (the first one starts it).
	MarkSarGenerationJobRateLimited(ctx context.Context, exec repositories.Executor, id uuid.UUID) (time.Duration, error)
	// ClearSarGenerationJobRateLimited ends the streak (a task succeeded).
	ClearSarGenerationJobRateLimited(ctx context.Context, exec repositories.Executor, id uuid.UUID) error
	CreateSarGenerationTasks(ctx context.Context, exec repositories.Executor, tasks []models.SarGenerationTask) error
	ListSarGenerationTasks(ctx context.Context, exec repositories.Executor, jobId uuid.UUID) ([]models.SarGenerationTask, error)
	// DeleteSarGenerationUnfinishedTasks deletes the given not-done tasks of the job
	// (one atomic statement, scoped to the job; done tasks are never deleted).
	DeleteSarGenerationUnfinishedTasks(ctx context.Context, exec repositories.Executor, jobId uuid.UUID, taskIds []uuid.UUID) error
	ClaimSarGenerationTask(ctx context.Context, exec repositories.Executor, id uuid.UUID,
		staleAfter time.Duration) (attempt int, ok bool, err error)
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
	// CaseContext is optional: without it no task receives a case context.
	CaseContext CaseContextReader
	Runner      TaskRunner
	Prompts     PromptSource
	// FormatGetter resolves a format key; defaults to formats.Get in the
	// constructor wiring (usecases_with_creds.go), injected in tests.
	FormatGetter func(core.FormatKey) (core.Format, bool)
	// Parallelism is SAR_GENERATION_PARALLELISM (<=0 -> DefaultParallelism).
	Parallelism int
	// StaleAfter: a running task started longer ago (database clock) is
	// presumed dead and can be claimed again (2 x SAR_GENERATION_TIMEOUT in
	// production).
	StaleAfter time.Duration
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
	caseContext        CaseContextReader
	runner             TaskRunner
	prompts            PromptSource
	formatGetter       func(core.FormatKey) (core.Format, bool)
	parallelism        int
	staleAfter         time.Duration
}

func NewOrchestrator(d Deps) *Orchestrator {
	o := &Orchestrator{
		executorFactory:    d.ExecutorFactory,
		transactionFactory: d.TransactionFactory,
		declarations:       d.Declarations,
		selection:          d.Selection,
		generation:         d.Generation,
		dataModels:         d.DataModels,
		caseContext:        d.CaseContext,
		runner:             d.Runner,
		prompts:            d.Prompts,
		formatGetter:       d.FormatGetter,
		parallelism:        d.Parallelism,
		staleAfter:         d.StaleAfter,
	}
	if o.parallelism <= 0 {
		o.parallelism = DefaultParallelism
	}
	if o.staleAfter <= 0 {
		o.staleAfter = 2 * DefaultTimeout
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
// rejections stay in the task output. Question paths are kept only within the
// task's allowed prefixes (and resolved); option fills only within the kept
// paths. A task of kind core.TaskKindCompute runs without the runner: Compute
// (Origin=OriginComputed, OverwriteNonManual, SkipMaxItems, RemoveItems first)
// is applied in one transaction. The same Compute also always runs at the end
// of a generation job (idempotent: it recomputes what later tasks changed). Review tasks
// write nothing and replace the open review_finding questions with their
// Findings; a review job only runs review specs. If Complete/Fail report false (superseded),
// the result is discarded and nothing is applied. Before launching the tasks
// of a step, every PromptPath must exist in Prompts, else the job is set
// failed with "prompt manquant : <chemin>". A technical task error ->
// FailSarGenerationTask, other running tasks finish, then RunJob returns the
// error (River retries); a models.LLMRateLimitedError must stay detectable
// with errors.Is. The job is set running at the start (pending or running ->
// running, bumping updated_at) and heartbeats (TouchSarGenerationJob) after
// each task. On success the job is set done.
func (o *Orchestrator) RunJob(ctx context.Context, jobId uuid.UUID) error {
	exec := o.executorFactory.NewExecutor()
	job, err := o.generation.GetSarGenerationJob(ctx, exec, jobId)
	if err != nil {
		return markGone(errors.Wrap(err, "cannot read generation job"))
	}
	if job.Status == models.SarJobStatusDone || job.Status == models.SarJobStatusFailed {
		return nil
	}
	ok, err := o.generation.SetSarGenerationJobStatus(ctx, exec, jobId, models.SarJobStatusRunning, "")
	if err != nil {
		return errors.Wrap(err, "cannot mark job running")
	}
	if !ok {
		return nil // finished in the meantime
	}
	decl, err := o.declarations.GetSarDeclaration(ctx, exec, job.DeclarationId)
	if err != nil {
		return markGone(errors.Wrap(err, "cannot read declaration"))
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
		o: o, job: job, declId: decl.Id, caseId: decl.CaseId, orgId: decl.OrganizationId, format: format,
		selection: selection, dataModel: dataModel,
	}
	run.rateLimited.Store(job.RateLimitedSince != nil)
	return run.loop(ctx)
}

func markGone(err error) error {
	if errors.Is(err, models.NotFoundError) {
		return errors.Mark(err, ErrJobGone)
	}
	return err
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
	caseId    uuid.UUID
	// caseCtx is the case context, built once per job on first need.
	caseCtxMu   sync.Mutex
	caseCtx     string
	caseCtxDone bool
	// applyMu serializes document writes inside this process; across
	// processes the FOR UPDATE lock does.
	applyMu sync.Mutex
	// rateLimited tells that the job row holds a rate-limit streak to clear
	// when a task succeeds.
	rateLimited atomic.Bool
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

		if r.job.Kind == models.SarJobKindReview {
			specs = r.reviewSpecsOnly(ctx, specs)
		}

		// A not-done task whose spec is no longer planned (its input changed since,
		// e.g. the analysis was edited between a failure and the resume) must never
		// run: it would apply a stale output next to the current one.
		planned := make(map[string]bool, len(specs))
		for _, s := range specs {
			planned[taskFromSpec(r.job.Id, s, r.selection).InputHash] = true
		}
		var stale []uuid.UUID
		kept := tasks[:0:0]
		for _, t := range tasks {
			if t.Status != models.SarJobStatusDone && !planned[t.InputHash] {
				stale = append(stale, t.Id)
				continue
			}
			kept = append(kept, t)
		}
		if len(stale) > 0 {
			if err := o.generation.DeleteSarGenerationUnfinishedTasks(ctx, exec, r.job.Id, stale); err != nil {
				return errors.Wrap(err, "cannot delete stale tasks")
			}
			utils.LoggerFromContext(ctx).InfoContext(ctx, "sar generation: stale tasks deleted",
				"job_id", r.job.Id, "count", len(stale))
			tasks = kept
		}

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
			// Concurrent planners may have inserted the same tasks (unique on
			// job and input hash): work on the rows that exist.
			if tasks, err = o.generation.ListSarGenerationTasks(ctx, exec, r.job.Id); err != nil {
				return errors.Wrap(err, "cannot list tasks")
			}
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
			return r.handleWaveError(ctx, err)
		}
		if !progress {
			return ErrTasksRunningElsewhere
		}
	}
}

// handleWaveError turns the error of a wave into the job's outcome. Errors a
// retry cannot fix (model not configured) and a rate limit lasting longer than
// RateLimitMaxDuration mark the job failed, in French, and are returned marked
// so that the worker cancels the River job; any other error is returned
// unchanged (River retries, a rate limit snoozes).
func (r *jobRun) handleWaveError(ctx context.Context, err error) error {
	exec := r.o.executorFactory.NewExecutor()
	dctx, cancel := detached(ctx)
	defer cancel()
	fail := func(msg string, marked error) error {
		if _, serr := r.o.generation.SetSarGenerationJobStatus(dctx, exec, r.job.Id, models.SarJobStatusFailed, msg); serr != nil {
			return errors.Wrap(err, "cannot fail job: "+serr.Error())
		}
		return marked
	}
	switch {
	case errors.Is(err, ErrLLMUnavailable):
		return fail("génération impossible : "+errors.UnwrapAll(err).Error(), err)
	case errors.Is(err, ErrTaskLimitReached):
		msg := ErrTaskLimitReached.Error()
		// The agent error carries the limit reason, in French (limitReachedMessage).
		if cause := errors.UnwrapAll(err).Error(); strings.HasPrefix(cause, limitMessagePrefix) {
			msg = cause
		}
		return fail(msg, err)
	case errors.Is(err, models.LLMRateLimitedError):
		r.rateLimited.Store(true)
		elapsed, merr := r.o.generation.MarkSarGenerationJobRateLimited(dctx, exec, r.job.Id)
		if merr != nil {
			utils.LoggerFromContext(ctx).WarnContext(ctx, "sar generation: cannot record rate limit",
				"job_id", r.job.Id, "error", merr.Error())
			return err
		}
		if elapsed > RateLimitMaxDuration {
			return fail(ErrRateLimitedTooLong.Error(), errors.Mark(errors.Wrap(err, ErrRateLimitedTooLong.Error()), ErrRateLimitedTooLong))
		}
	}
	return err
}

// reviewSpecsOnly drops, with a warning, the non-review specs planned for a
// review job: a review never writes the document.
func (r *jobRun) reviewSpecsOnly(ctx context.Context, specs []core.TaskSpec) []core.TaskSpec {
	kept := specs[:0:0]
	for _, s := range specs {
		if s.Kind != core.TaskKindReview {
			utils.LoggerFromContext(ctx).WarnContext(ctx, "sar generation: non-review task refused in a review job",
				"job_id", r.job.Id, "kind", s.Kind, "target", string(s.TargetPath))
			continue
		}
		kept = append(kept, s)
	}
	return kept
}

// checkPrompt fails the job when the prompt of a spec, or the instructions of
// the format, does not exist.
func (r *jobRun) checkPrompt(ctx context.Context, s core.TaskSpec) error {
	missing := ""
	switch {
	case s.PromptPath != "" && !r.o.prompts.Exists(s.PromptPath):
		missing = s.PromptPath
	default:
		// The format's own instructions are prepended to every step prompt.
		if fi, ok := r.format.(FormatInstructions); ok {
			if p := fi.InstructionsPromptPath(); p != "" && !r.o.prompts.Exists(p) {
				missing = p
			}
		}
	}
	if missing == "" {
		return nil
	}
	msg := "prompt manquant : " + missing
	if err := r.failJob(ctx, msg); err != nil {
		return err
	}
	return errors.Mark(errors.New(msg), ErrPromptMissing)
}

// failJob sets the job failed with msg, on a context that survives the
// cancellation of ctx.
func (r *jobRun) failJob(ctx context.Context, msg string) error {
	dctx, cancel := detached(ctx)
	defer cancel()
	if _, err := r.o.generation.SetSarGenerationJobStatus(dctx, r.o.executorFactory.NewExecutor(),
		r.job.Id, models.SarJobStatusFailed, msg); err != nil {
		return errors.Wrap(err, "cannot fail job")
	}
	return nil
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
		wg        sync.WaitGroup
		mu        sync.Mutex
		claimed   bool
		firstErr  error
		rateErr   error
		limitErr  error
		promptErr error
		sem       = make(chan struct{}, r.o.parallelism)
		failed    bool
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
				if errors.Is(err, ErrTaskLimitReached) && limitErr == nil {
					limitErr = err
				}
				if errors.Is(err, ErrPromptMissing) && promptErr == nil {
					promptErr = err
				}
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
	// A missing prompt ends the job for good: it wins over everything else (no
	// retry, no snooze). A limit failure of an essential task ends the job too:
	// it wins over a rate limit (snooze) and over the first error (retry).
	if promptErr != nil {
		return claimed, promptErr
	}
	if limitErr != nil {
		return claimed, limitErr
	}
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
	attempt, ok, err := o.generation.ClaimSarGenerationTask(ctx, exec, rt.task.Id, o.staleAfter)
	if err != nil {
		return false, errors.Wrap(err, "cannot claim task")
	}
	if !ok {
		return false, nil
	}
	claimed = true
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

	var (
		out        core.TaskOutput
		superseded bool
	)
	apply := func(tx repositories.Transaction) error {
		var err error
		if rt.spec.Kind == core.TaskKindCompute {
			superseded, err = r.applyCompute(ctx, tx, rt, attempt)
		} else {
			superseded, err = r.applyOutput(ctx, tx, rt, attempt, out)
		}
		return err
	}
	if rt.spec.Kind != core.TaskKindCompute {
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
			AnalystText: analystText(decl),
		}
		if rt.spec.NeedsCaseContext {
			cc, err := r.caseContextText(ctx)
			if err != nil {
				return true, fail(err, nil)
			}
			tc.CaseContext = cc
		}
		task := rt.task
		task.Attempts = attempt
		var runErr error
		out, runErr = o.runner.Run(ctx, tc, task, rt.spec)
		if runErr != nil {
			failed := fail(runErr, out.Trace)
			if errors.Is(runErr, ErrPromptMissing) {
				// A prompt that disappeared after the plan (checked before the
				// tasks start): same outcome as the check, the job is failed and
				// the worker does not retry.
				if jerr := r.failJob(ctx, runErr.Error()); jerr != nil {
					return true, errors.Wrap(failed, jerr.Error())
				}
			}
			return true, failed
		}
	}

	applyErr := func() error {
		r.applyMu.Lock()
		defer r.applyMu.Unlock()
		return o.transactionFactory.Transaction(ctx, apply)
	}()
	if applyErr != nil {
		return true, fail(applyErr, out.Trace)
	}
	if superseded {
		return true, nil
	}
	// A task went through: the provider is not blocking the job any more.
	if r.rateLimited.CompareAndSwap(true, false) {
		if err := o.generation.ClearSarGenerationJobRateLimited(ctx, exec, r.job.Id); err != nil {
			utils.LoggerFromContext(ctx).WarnContext(ctx, "sar generation: cannot clear rate limit",
				"job_id", r.job.Id, "error", err.Error())
		}
	}
	// Heartbeat: a live job is told apart from a dead one by updated_at.
	// The output is applied: a failed heartbeat must not fail the task.
	if err := o.generation.TouchSarGenerationJob(ctx, exec, r.job.Id); err != nil {
		utils.LoggerFromContext(ctx).WarnContext(ctx, "sar generation: heartbeat failed",
			"job_id", r.job.Id, "error", err.Error())
	}
	return true, nil
}

// stillRunning reads the job status inside the transaction, after the
// declaration row lock: nothing may be written for a job that is not running
// any more (failed by a writer for staleness, or finished).
func (r *jobRun) stillRunning(ctx context.Context, tx repositories.Transaction) (bool, error) {
	job, err := r.o.generation.GetSarGenerationJob(ctx, tx, r.job.Id)
	if err != nil {
		return false, errors.Wrap(err, "cannot read job")
	}
	return job.Status == models.SarJobStatusRunning, nil
}

// taskResult is the persisted output of a task.
type taskResult struct {
	Applied   []core.Path    `json:"applied"`
	Rejected  []rejectedView `json:"rejected"`
	Conflicts []conflictView `json:"conflicts"`
	Questions int            `json:"questions"`
	Findings  int            `json:"findings"`
	// Wrote tells whether applying the task changed the document. The selection
	// stays editable after a failed job whose done tasks all wrote nothing.
	Wrote bool `json:"wrote"`
	// Structured is the structured output of the task (e.g. the understanding
	// note), kept so that the planner can hand it to the later steps.
	Structured json.RawMessage `json:"structured,omitempty"`
}

// CodeUnresolvedPath marks a question path still holding a provisional key.
const CodeUnresolvedPath = "unresolved_path"

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
	if running, err := r.stillRunning(ctx, tx); err != nil {
		return false, err
	} else if !running {
		// Job failed or finished meanwhile: nothing written (the transaction
		// rolls back); runTask releases the task and stops the run.
		return false, ErrJobNotRunning
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

	var structuredRejected []rejectedView
	if !isReview {
		if len(out.Structured) > 0 {
			// Applied to a copy first: a structured output citing objects outside
			// the frozen selection is rejected as a whole, nothing of it applied.
			cand := doc.Clone()
			sctx := StructuredContextFor(r.selection, r.dataModel)
			sctx.AnalystText = analystText(decl)
			if err := r.format.ApplyStructured(rt.spec.Kind, cand, out.Structured, sctx); err != nil {
				return false, errors.Wrap(err, "invalid structured output")
			}
			if bad := sourcesOutsideSelection(doc, cand, newScope(r.selection)); len(bad) > 0 {
				structuredRejected = append(structuredRejected, rejectedView{
					Path: rt.spec.TargetPath, Code: core.CodeSourceOutOfScope,
					Message: "structured output rejected: " + outsideSelectionMessage(bad)})
			} else {
				// A structured output that leaves the document identical (a
				// note kept in the task output only) writes nothing and bumps
				// no version.
				before, err := json.Marshal(doc)
				if err != nil {
					return false, errors.Wrap(err, "cannot encode document")
				}
				after, err := json.Marshal(cand)
				if err != nil {
					return false, errors.Wrap(err, "cannot encode document")
				}
				if !bytes.Equal(before, after) {
					doc, changed = cand, true
				}
				if sq, ok := r.format.(core.StructuredQuestioner); ok {
					out.Questions = append(slices.Clone(out.Questions), sq.StructuredQuestions(rt.spec.Kind, out.Structured, cand)...)
				}
			}
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
		// Built once: it reads the document at call time, which follows the fills.
		allowedSources := AllowedSources(doc, r.selection, r.dataModel, rt.spec.Inputs)
		res = core.ApplyFills(r.format.Spec(), doc, fills, core.ApplyOptions{
			AllowedPrefixes: allowed, Overwrite: core.OverwriteNonManual,
			AllowedSources: allowedSources,
		})
		changed = changed || len(res.Applied) > 0
		result = resultFromApply(res)
		result.Rejected = append(result.Rejected, structuredRejected...)
		if len(out.Structured) > 0 && len(structuredRejected) == 0 {
			result.Structured = out.Structured
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
			// The model's paths are not trusted either: resolved, within the
			// task's part and fully created, else dropped from the question.
			paths := []string{}
			kept := make([]core.Path, 0, len(d.Paths))
			for _, p := range d.Paths {
				rp := res.Rewrite(p)
				switch {
				case rp.HasProvisionalKey() && !newItemPathKnown(r.format.Spec(), rp):
					result.Rejected = append(result.Rejected, rejectedView{
						Path: rp, Code: CodeUnresolvedPath, Message: "question: path to an element that was not created"})
				case !core.PathAllowed(rp, allowed):
					result.Rejected = append(result.Rejected, rejectedView{
						Path: rp, Code: core.CodeOutOfScope, Message: "question: path outside the part assigned to this task"})
				default:
					// A path still holding a provisional key designates an item
					// the analyst creates by choosing an option: kept as is.
					kept = append(kept, rp)
					paths = append(paths, string(rp))
				}
			}
			k := questionKey(paths, d.Text)
			if seen[k] {
				continue
			}
			seen[k] = true
			opts := make([]core.QuestionOption, 0, len(d.Options))
			createsItem, removalRejected := false, false
			for i, op := range d.Options {
				op.Fills = nil
				if op.RemoveItem != "" {
					// The item the option removes must be one of the question's
					// own paths and exist in the document the output produced.
					op.RemoveItem = res.Rewrite(op.RemoveItem)
					if !core.PathAllowed(op.RemoveItem, kept) || !documentHasItem(doc, op.RemoveItem) {
						result.Rejected = append(result.Rejected, rejectedView{
							Path: op.RemoveItem, Code: core.CodeOutOfScope,
							Message: "question option: the item to remove is outside the paths of the question"})
						removalRejected = true
						continue
					}
					// A removing option comes with its own « keep » sibling, which
					// has no fill: it must survive the filter below.
					createsItem = true
				}
				for _, f := range d.Options[i].Fills {
					f.Path = res.Rewrite(f.Path)
					if (f.Path.HasProvisionalKey() && !newItemPathKnown(r.format.Spec(), f.Path)) ||
						!core.PathAllowed(f.Path, kept) {
						result.Rejected = append(result.Rejected, rejectedView{
							Path: f.Path, Code: core.CodeOutOfScope,
							Message: "question option: value outside the fields of the question"})
						continue
					}
					if !sourcesAllowed(f, allowedSources(f.Path)) {
						result.Rejected = append(result.Rejected, rejectedView{
							Path: f.Path, Code: core.CodeSourceOutOfScope,
							Message: "question option: source outside the scope allowed for this field"})
						continue
					}
					// Like the task's own fills: the model never picks the origin.
					f.Origin, f.TaskID = core.OriginLLM, &taskId
					createsItem = createsItem || f.Path.HasProvisionalKey()
					op.Fills = append(op.Fills, f)
				}
				if len(d.Options[i].Fills) > 0 && len(op.Fills) == 0 {
					continue // it came with values that were all rejected: never a decline
				}
				opts = append(opts, op)
			}
			if removalRejected {
				// Never a question whose removing option was refused: its keep
				// sibling would be dropped and a blocking question left without
				// any option.
				continue
			}
			// An option that arrived with no fill at all cannot be chosen, unless it declines
			// the addition of a new list item proposed by a sibling option.
			if !createsItem {
				opts = slices.DeleteFunc(opts, func(op core.QuestionOption) bool { return len(op.Fills) == 0 })
			}
			var rawOpts []byte
			if len(opts) > 0 {
				if rawOpts, err = json.Marshal(opts); err != nil {
					return false, errors.Wrap(err, "cannot encode question options")
				}
			}
			severity := ""
			if d.Blocking {
				severity = models.SarQuestionSeverityBlocking
			}
			qs = append(qs, models.SarQuestion{
				Id: uuid.New(), DeclarationId: r.declId, TaskId: &taskId,
				Kind: models.SarQuestionKindQuestion, Severity: severity, Paths: paths, Text: d.Text,
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

	result.Wrote = changed
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

func resultFromApply(res core.ApplyResult) taskResult {
	result := taskResult{Applied: res.Applied}
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
	return result
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

// sourcesAllowed tells whether every source of f is among the allowed objects.
func sourcesAllowed(f core.Fill, allowed []core.ObjectRef) bool {
	for _, s := range f.Sources {
		if !slices.Contains(allowed, s.Object) {
			return false
		}
	}
	return true
}

// computeInto derives the computed fields into doc (RemoveItems first, then
// the fills as OriginComputed, OverwriteNonManual, SkipMaxItems). changed
// tells whether the document was modified.
func (r *jobRun) computeInto(doc *core.Document) (res core.ApplyResult, changed bool) {
	return core.ComputeInto(r.format, doc)
}

// applyCompute runs a compute task in tx (no runner): the declaration is read
// FOR UPDATE, computed in memory, the task completed with the attempt guard,
// then the document saved. superseded as in applyOutput.
func (r *jobRun) applyCompute(ctx context.Context, tx repositories.Transaction, rt readyTask, attempt int,
) (superseded bool, err error) {
	o := r.o
	decl, err := o.declarations.GetSarDeclarationForUpdate(ctx, tx, r.declId)
	if err != nil {
		return false, errors.Wrap(err, "cannot lock declaration")
	}
	if running, err := r.stillRunning(ctx, tx); err != nil {
		return false, err
	} else if !running {
		// Job failed or finished meanwhile: nothing written (the transaction
		// rolls back); runTask releases the task and stops the run.
		return false, ErrJobNotRunning
	}
	doc, err := decodeDocument(decl.Document)
	if err != nil {
		return false, err
	}
	res, changed := r.computeInto(doc)
	result := resultFromApply(res)
	result.Wrote = changed
	rawOut, err := json.Marshal(result)
	if err != nil {
		return false, errors.Wrap(err, "cannot encode task output")
	}
	ok, err := o.generation.CompleteSarGenerationTask(ctx, tx, rt.task.Id, attempt, rawOut, nil)
	if err != nil {
		return false, errors.Wrap(err, "cannot complete task")
	}
	if !ok {
		return true, nil
	}
	if changed {
		return false, r.saveDocument(ctx, tx, decl, doc)
	}
	return false, nil
}

// finish runs Compute (generation jobs, always: it is idempotent, and it
// recomputes what the tasks that ran after a planned compute task changed, e.g.
// corrections) and marks the job done.
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
				if running, err := r.stillRunning(ctx, tx); err != nil {
					return err
				} else if !running {
					return nil // job failed meanwhile: nothing written
				}
				doc, err := decodeDocument(decl.Document)
				if err != nil {
					return err
				}
				res, changed := r.computeInto(doc)
				if len(res.Rejected) > 0 || len(res.Conflicts) > 0 {
					utils.LoggerFromContext(ctx).WarnContext(ctx, "sar generation: computed fills not applied",
						"job_id", r.job.Id, "rejected", len(res.Rejected), "conflicts", len(res.Conflicts),
						"first_rejected", firstRejected(res))
				}
				// Required coded fields the generation left empty get a question
				// that offers the unknown code of their list.
				open, err := o.generation.ListSarQuestions(ctx, tx, r.declId, models.SarQuestionStatusOpen)
				if err != nil {
					return errors.Wrap(err, "cannot list questions")
				}
				if qs := missingCodeQuestions(r.format, doc, r.declId, open); len(qs) > 0 {
					if err := o.generation.CreateSarQuestions(ctx, tx, qs); err != nil {
						return errors.Wrap(err, "cannot create questions")
					}
				}
				if !changed {
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
	// false: the job was finished meanwhile (nothing left to do).
	if _, err := o.generation.SetSarGenerationJobStatus(dctx, exec, r.job.Id, models.SarJobStatusDone, ""); err != nil {
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

// documentHasItem tells whether the keyed item path `a.b[k]` is listed in doc.
func documentHasItem(doc *core.Document, item core.Path) bool {
	str := string(item)
	i := strings.LastIndex(str, "[")
	if i < 0 || !strings.HasSuffix(str, "]") {
		return false
	}
	name, key := str[:i], str[i+1:len(str)-1]
	if j := strings.LastIndex(name, "."); j >= 0 {
		name = name[j+1:]
	}
	return slices.Contains(doc.Items[item.Parent().Child(name)], key)
}

// marshalOptions encodes the options of a stored question.
func marshalOptions(opts []core.QuestionOption) ([]byte, error) {
	raw, err := json.Marshal(opts)
	return raw, errors.Wrap(err, "cannot encode question options")
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

// newItemPathKnown tells whether p, a path holding provisional keys, designates
// a new item of repeatable containers that exist in the spec (every segment
// with a provisional key is a repeatable node) and a known node.
func newItemPathKnown(spec *core.Spec, p core.Path) bool {
	if spec == nil {
		return false
	}
	if _, err := core.ParsePath(string(p)); err != nil {
		return false
	}
	segs := strings.Split(string(p), ".")
	tmpl := make([]string, 0, len(segs))
	for _, seg := range segs {
		name, key := seg, ""
		if j := strings.Index(seg, "["); j >= 0 {
			name, key = seg[:j], seg[j+1:len(seg)-1]
			tmpl = append(tmpl, name+"[]")
		} else {
			tmpl = append(tmpl, name)
		}
		if strings.HasPrefix(key, "+") {
			n := spec.NodeAt(strings.Join(tmpl, "."))
			if n == nil || !n.IsRepeatable() {
				return false
			}
		}
	}
	n := spec.NodeAt(strings.Join(tmpl, "."))
	return n != nil && n.Template != ""
}

// caseContextText returns the case context of the job, read once with the
// declaration's organization and case (never from a model output).
func (r *jobRun) caseContextText(ctx context.Context) (string, error) {
	if r.o.caseContext == nil {
		return "", nil
	}
	r.caseCtxMu.Lock()
	defer r.caseCtxMu.Unlock()
	if r.caseCtxDone {
		return r.caseCtx, nil
	}
	text, err := buildCaseContext(ctx, r.o.executorFactory.NewExecutor(), r.o.caseContext, r.orgId, r.caseId)
	if err != nil {
		return "", err // not cached: the next task tries again
	}
	r.caseCtx, r.caseCtxDone = text, true
	return text, nil
}
