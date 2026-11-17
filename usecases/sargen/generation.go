package sargen

import (
	"bytes"
	"context"
	"encoding/json"
	"slices"
	"strings"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/google/uuid"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/sargen/core"
	"github.com/checkmarble/marble-backend/utils"
)

var (
	// ErrGenerationInProgress (409) is returned by every write on a declaration
	// while one of its jobs is pending or running.
	ErrGenerationInProgress = errors.Wrap(models.ConflictError, "une génération est en cours")
	// ErrSelectionFrozen (409) is returned by UpdateSelection once a generate
	// job is done for the declaration.
	ErrSelectionFrozen = errors.Wrap(models.ConflictError,
		"la sélection ne peut plus changer après la génération ; créez une nouvelle déclaration")
	// ErrAlreadyGenerated (409) is returned by StartGeneration when a generate
	// job is already done (no regeneration in V1).
	ErrAlreadyGenerated = errors.Wrap(models.ConflictError,
		"cette déclaration a déjà été générée ; la regénération n'est pas disponible")
)

// SarGenerationRepository is the subset of the jobs/tasks/questions repository
// used by the usecase (satisfied by *repositories.MarbleDbRepository).
type SarGenerationRepository interface {
	// CreateSarGenerationJob returns models.ErrSarJobAlreadyActive when a
	// pending/running job exists for the declaration (partial unique index).
	CreateSarGenerationJob(ctx context.Context, exec repositories.Executor, j models.SarGenerationJob) error
	GetSarGenerationJob(ctx context.Context, exec repositories.Executor, id uuid.UUID) (models.SarGenerationJob, error)
	// ListSarGenerationJobs returns the jobs of a declaration, newest first.
	ListSarGenerationJobs(ctx context.Context, exec repositories.Executor, declarationId uuid.UUID) ([]models.SarGenerationJob, error)
	// SetSarGenerationJobStatus also bumps updated_at; it may return
	// models.ErrSarJobAlreadyActive when moving a job back to pending.
	SetSarGenerationJobStatus(ctx context.Context, exec repositories.Executor, id uuid.UUID,
		status models.SarJobStatus, errMsg string) error
	ListSarGenerationTasks(ctx context.Context, exec repositories.Executor, jobId uuid.UUID) ([]models.SarGenerationTask, error)
	ListSarQuestions(ctx context.Context, exec repositories.Executor, declarationId uuid.UUID, status string) ([]models.SarQuestion, error)
	GetSarQuestion(ctx context.Context, exec repositories.Executor, id uuid.UUID) (models.SarQuestion, error)
	CloseSarQuestions(ctx context.Context, exec repositories.Executor, ids []uuid.UUID, status string) error
}

// GenerationEnqueuer is satisfied by repositories.TaskQueueRepository: it
// inserts the River job inside the caller's transaction.
type GenerationEnqueuer interface {
	EnqueueSarGenerationTask(ctx context.Context, tx repositories.Transaction,
		organizationId uuid.UUID, jobId, declarationId uuid.UUID) error
}

// AiAccessChecker tells whether the organization may use the AI features. It
// returns models.ForbiddenError (wrapped, French message) when not allowed.
type AiAccessChecker interface {
	CheckAiAccess(ctx context.Context, orgId uuid.UUID) error
}

// caseReviewAiAccess is what the case review uses (ai_agent.AiAgentUsecase):
// the organization feature access CaseAiAssist (license / entitlement).
type caseReviewAiAccess interface {
	HasAiCaseReviewEnabled(ctx context.Context, orgId uuid.UUID) (bool, error)
}

// AiAccessFromCaseReview adapts the case review access check to AiAccessChecker.
func AiAccessFromCaseReview(r caseReviewAiAccess) AiAccessChecker { return aiAccessAdapter{r} }

type aiAccessAdapter struct{ r caseReviewAiAccess }

func (a aiAccessAdapter) CheckAiAccess(ctx context.Context, orgId uuid.UUID) error {
	ok, err := a.r.HasAiCaseReviewEnabled(ctx, orgId)
	if err != nil {
		return errors.Wrap(err, "cannot check AI access")
	}
	if !ok {
		return errors.Wrap(models.ForbiddenError, "la génération par IA n'est pas disponible pour votre organisation")
	}
	return nil
}

// TaskSummary is the analyst view of a generation task (no input, output or
// trace).
type TaskSummary struct {
	Id         uuid.UUID
	Kind       string
	TargetPath string
	Status     models.SarJobStatus
	Attempts   int
	Error      string
	StartedAt  *time.Time
	FinishedAt *time.Time
}

// JobState is what GetJob returns.
//
// Done / Total count tasks (status done / all tasks of the job; the orchestrator
// plans tasks in waves, so Total can grow while the job runs). LastError is the
// job error, else the error of the most recently failed task, else "".
//
// Attempt / MaxAttempts: no River reader exists, so Attempt is the highest
// task attempt count of the job (0 before any task started) and MaxAttempts is
// models.SarGenerationMaxAttempts (the River MaxAttempts fixed at insert time).
type JobState struct {
	Job         models.SarGenerationJob
	Tasks       []TaskSummary
	Done, Total int
	LastError   string
	Attempt     int
	MaxAttempts int
}

// StartGeneration starts the `generate` job of a declaration.
//
//   - same AI access check as the case review (AiAccessChecker; ForbiddenError);
//   - 409 ErrAlreadyGenerated when a `generate` job is already done (no
//     regeneration in V1); 409 models.ErrSarJobAlreadyActive when one is active;
//   - 400 when declaration.Analysis is empty or the selection is empty;
//   - 400 « réglage de l'organisation incomplet » listing the missing or
//     invalid paths when requiredSettingPaths(format, stored values) are not all
//     present and valid (see requiredSettingPaths for the exact rule); no job is
//     created;
//   - re-applies the org settings to the document (OriginOrgSetting,
//     core.OverwriteNonManual) so settings filled after creation are used;
//     the document is saved with a version bump only if it changed;
//   - creates the job (status pending, kind generate, created_by) and enqueues
//     it (GenerationEnqueuer) in the same transaction, under the declaration
//     row lock (GetSarDeclarationForUpdate).
func (uc *Usecase) StartGeneration(ctx context.Context, id uuid.UUID) (models.SarGenerationJob, error) {
	exec := uc.executorFactory.NewExecutor()
	d, err := uc.loadDeclaration(ctx, exec, id)
	if err != nil {
		return models.SarGenerationJob{}, err
	}
	if err := uc.aiAccess.CheckAiAccess(ctx, d.OrganizationId); err != nil {
		return models.SarGenerationJob{}, err
	}
	var job models.SarGenerationJob
	err = uc.transactionFactory.Transaction(ctx, func(tx repositories.Transaction) error {
		locked, err := uc.repository.GetSarDeclarationForUpdate(ctx, tx, id)
		if err != nil {
			return err
		}
		if err := uc.checkDeclarationAccess(ctx, tx, locked); err != nil {
			return err
		}
		jobs, err := uc.generation.ListSarGenerationJobs(ctx, tx, id)
		if err != nil {
			return err
		}
		if hasDoneGenerate(jobs) {
			return ErrAlreadyGenerated
		}
		if hasActiveJob(jobs) {
			return models.ErrSarJobAlreadyActive
		}
		if strings.TrimSpace(locked.Analysis) == "" {
			return errors.Wrap(models.BadParameterError, "l'analyse est vide : renseignez-la avant de générer")
		}
		selection, err := uc.selectionRepo.ListSarSelection(ctx, tx, id)
		if err != nil {
			return err
		}
		if len(selection) == 0 {
			return errors.Wrap(models.BadParameterError, "la sélection est vide : sélectionnez des objets avant de générer")
		}
		format, err := uc.formatOf(locked)
		if err != nil {
			return err
		}
		spec := format.Spec()
		settings, err := uc.repository.GetSarOrgFormatSettings(ctx, tx, locked.OrganizationId, format.Key().Family)
		if err != nil {
			return err
		}
		values := map[string]string{}
		if settings != nil {
			values = settings.Values
		}
		var bad []string
		for _, p := range requiredSettingPaths(format, values) {
			v, ok := values[string(p)]
			if !ok || strings.TrimSpace(v) == "" {
				bad = append(bad, string(p)+" (manquant)")
				continue
			}
			if issue := core.ValidateValue(spec, p, v); issue != nil && issue.Blocks() {
				bad = append(bad, string(p)+" (invalide)")
			}
		}
		if len(bad) > 0 {
			return errors.Wrapf(models.BadParameterError, "réglage de l'organisation incomplet : %s", strings.Join(bad, ", "))
		}

		// Re-apply the organization settings to the document.
		doc, err := decodeDocument(locked.Document)
		if err != nil {
			return err
		}
		before, err := json.Marshal(doc)
		if err != nil {
			return errors.Wrap(err, "cannot encode declaration document")
		}
		paths := make([]string, 0, len(values))
		for p := range values {
			paths = append(paths, p)
		}
		slices.Sort(paths)
		fills := make([]core.Fill, 0, len(paths))
		for _, p := range paths {
			fills = append(fills, core.Fill{Path: core.Path(p), Value: values[p], Origin: core.OriginOrgSetting})
		}
		logger := utils.LoggerFromContext(ctx)
		for _, r := range core.ApplyFills(spec, doc, fills, core.ApplyOptions{Overwrite: core.OverwriteNonManual}).Rejected {
			logger.WarnContext(ctx, "sar generation: org setting rejected",
				"path", string(r.Fill.Path), "code", r.Issue.Code)
		}
		after, err := json.Marshal(doc)
		if err != nil {
			return errors.Wrap(err, "cannot encode declaration document")
		}
		if !bytes.Equal(before, after) {
			if _, err := uc.repository.UpdateSarDeclarationDocument(ctx, tx, id, locked.Version, after,
				locked.Analysis, locked.ExtraInfo); err != nil {
				return err
			}
		}

		job, err = uc.createAndEnqueue(ctx, tx, locked, models.SarJobKindGenerate)
		return err
	})
	if err != nil {
		return models.SarGenerationJob{}, err
	}
	return job, nil
}

// createAndEnqueue inserts a pending job and enqueues it, in tx.
func (uc *Usecase) createAndEnqueue(ctx context.Context, tx repositories.Transaction,
	d models.SarDeclaration, kind models.SarJobKind,
) (models.SarGenerationJob, error) {
	j := models.SarGenerationJob{
		Id: pure_utils.NewId(), DeclarationId: d.Id, Kind: kind,
		Status: models.SarJobStatusPending, CreatedBy: actorId(ctx),
	}
	if err := uc.generation.CreateSarGenerationJob(ctx, tx, j); err != nil {
		return models.SarGenerationJob{}, err
	}
	if err := uc.enqueuer.EnqueueSarGenerationTask(ctx, tx, d.OrganizationId, j.Id, d.Id); err != nil {
		return models.SarGenerationJob{}, errors.Wrap(err, "cannot enqueue generation job")
	}
	return uc.generation.GetSarGenerationJob(ctx, tx, j.Id)
}

func hasActiveJob(jobs []models.SarGenerationJob) bool {
	return slices.ContainsFunc(jobs, func(j models.SarGenerationJob) bool { return isActiveJob(j.Status) })
}

func isActiveJob(s models.SarJobStatus) bool {
	return s == models.SarJobStatusPending || s == models.SarJobStatusRunning
}

func hasDoneGenerate(jobs []models.SarGenerationJob) bool {
	return slices.ContainsFunc(jobs, func(j models.SarGenerationJob) bool {
		return j.Kind == models.SarJobKindGenerate && j.Status == models.SarJobStatusDone
	})
}

// isStale: running with no update for more than 2 x the job timeout.
func (uc *Usecase) isStale(j models.SarGenerationJob) bool {
	return j.Status == models.SarJobStatusRunning && uc.now().Sub(j.UpdatedAt) > 2*uc.jobTimeout
}

// GetJob returns the progress of a job. Access is checked through the job's
// declaration (loadDeclaration); another organization -> ForbiddenError.
func (uc *Usecase) GetJob(ctx context.Context, jobId uuid.UUID) (JobState, error) {
	exec := uc.executorFactory.NewExecutor()
	job, err := uc.generation.GetSarGenerationJob(ctx, exec, jobId)
	if err != nil {
		return JobState{}, err
	}
	if _, err := uc.loadDeclaration(ctx, exec, job.DeclarationId); err != nil {
		return JobState{}, err
	}
	tasks, err := uc.generation.ListSarGenerationTasks(ctx, exec, jobId)
	if err != nil {
		return JobState{}, err
	}
	st := JobState{Job: job, Total: len(tasks), MaxAttempts: models.SarGenerationMaxAttempts}
	lastTaskErr := ""
	for _, t := range tasks {
		st.Tasks = append(st.Tasks, TaskSummary{
			Id: t.Id, Kind: t.Kind, TargetPath: t.TargetPath, Status: t.Status,
			Attempts: t.Attempts, Error: t.Error, StartedAt: t.StartedAt, FinishedAt: t.FinishedAt,
		})
		if t.Status == models.SarJobStatusDone {
			st.Done++
		}
		if t.Attempts > st.Attempt {
			st.Attempt = t.Attempts
		}
		if t.Status == models.SarJobStatusFailed && t.Error != "" {
			lastTaskErr = t.Error
		}
	}
	st.LastError = job.Error
	if uc.isStale(job) {
		st.Job.Status = models.SarJobStatusFailed
		if st.LastError == "" {
			st.LastError = "la génération ne répond plus"
		}
	}
	if st.LastError == "" {
		st.LastError = lastTaskErr
	}
	return st, nil
}

// ResumeJob puts a job back to pending and re-enqueues it (same transaction),
// only when it is `failed`, or `running` with no update (job updated_at) for
// more than 2 x the job timeout (Deps.JobTimeout); otherwise 409. Same AI
// access check as StartGeneration. A failed job whose declaration already has
// another active job conflicts (models.ErrSarJobAlreadyActive).
func (uc *Usecase) ResumeJob(ctx context.Context, jobId uuid.UUID) (models.SarGenerationJob, error) {
	exec := uc.executorFactory.NewExecutor()
	job, err := uc.generation.GetSarGenerationJob(ctx, exec, jobId)
	if err != nil {
		return models.SarGenerationJob{}, err
	}
	d, err := uc.loadDeclaration(ctx, exec, job.DeclarationId)
	if err != nil {
		return models.SarGenerationJob{}, err
	}
	if err := uc.aiAccess.CheckAiAccess(ctx, d.OrganizationId); err != nil {
		return models.SarGenerationJob{}, err
	}
	var out models.SarGenerationJob
	err = uc.transactionFactory.Transaction(ctx, func(tx repositories.Transaction) error {
		locked, err := uc.repository.GetSarDeclarationForUpdate(ctx, tx, d.Id)
		if err != nil {
			return err
		}
		if err := uc.checkDeclarationAccess(ctx, tx, locked); err != nil {
			return err
		}
		cur, err := uc.generation.GetSarGenerationJob(ctx, tx, jobId)
		if err != nil {
			return err
		}
		if cur.Status != models.SarJobStatusFailed && !uc.isStale(cur) {
			return errors.Wrap(models.ConflictError, "cette génération ne peut pas être relancée")
		}
		if err := uc.generation.SetSarGenerationJobStatus(ctx, tx, jobId, models.SarJobStatusPending, ""); err != nil {
			return err
		}
		if err := uc.enqueuer.EnqueueSarGenerationTask(ctx, tx, locked.OrganizationId, jobId, locked.Id); err != nil {
			return errors.Wrap(err, "cannot enqueue generation job")
		}
		out, err = uc.generation.GetSarGenerationJob(ctx, tx, jobId)
		return err
	})
	if err != nil {
		return models.SarGenerationJob{}, err
	}
	return out, nil
}

// StartReview starts a `review` job (the format plans only the `review` task,
// PlanState.JobKind == "review"). Same AI access check; 409 (models.ConflictError)
// « aucune génération terminée » when the declaration has no done `generate`
// job; allowed any number of times; never writes the document (the version does
// not change). Job and enqueue in the same transaction.
func (uc *Usecase) StartReview(ctx context.Context, id uuid.UUID) (models.SarGenerationJob, error) {
	exec := uc.executorFactory.NewExecutor()
	d, err := uc.loadDeclaration(ctx, exec, id)
	if err != nil {
		return models.SarGenerationJob{}, err
	}
	if err := uc.aiAccess.CheckAiAccess(ctx, d.OrganizationId); err != nil {
		return models.SarGenerationJob{}, err
	}
	var job models.SarGenerationJob
	err = uc.transactionFactory.Transaction(ctx, func(tx repositories.Transaction) error {
		locked, err := uc.repository.GetSarDeclarationForUpdate(ctx, tx, id)
		if err != nil {
			return err
		}
		if err := uc.checkDeclarationAccess(ctx, tx, locked); err != nil {
			return err
		}
		jobs, err := uc.generation.ListSarGenerationJobs(ctx, tx, id)
		if err != nil {
			return err
		}
		if hasActiveJob(jobs) {
			return models.ErrSarJobAlreadyActive
		}
		if !hasDoneGenerate(jobs) {
			return errors.Wrap(models.ConflictError, "aucune génération terminée : lancez d'abord la génération")
		}
		job, err = uc.createAndEnqueue(ctx, tx, locked, models.SarJobKindReview)
		return err
	})
	if err != nil {
		return models.SarGenerationJob{}, err
	}
	return job, nil
}

// requireNoActiveJob returns ErrGenerationInProgress while a job of the
// declaration is pending or running. Called inside the writers' transactions
// after the row lock.
func (uc *Usecase) requireNoActiveJob(ctx context.Context, exec repositories.Executor, declarationId uuid.UUID) error {
	jobs, err := uc.generation.ListSarGenerationJobs(ctx, exec, declarationId)
	if err != nil {
		return err
	}
	if hasActiveJob(jobs) {
		return ErrGenerationInProgress
	}
	return nil
}

// requiredSettingPaths returns the organization setting paths that must be
// filled and valid before a generation starts, sorted.
//
// Rule (generic, spec-driven): walk every NatureSetting field leaf outside a
// repeatable container; the leaf is required when its own Min >= 1 AND every
// ancestor group is "present", i.e. the group has Min >= 1 or at least one
// stored setting value lives under it (a stored value shows the organization
// chose to fill that optional group; then its mandatory leaves are required).
// PLUS the paths of core.SettingsRequirer.RequiredSettings() when the format
// implements it.
//
// On TRACFIN 1.4.0 the generic part gives organisme.* (profession, libelle,
// numIdPro, adresse.nomVoie/codePostal/ville/pays/tel) and, only when an
// optional group is filled, its mandatory leaves (contact.coordonnees.*). It does NOT
// give declarant.coordonnees.* (group min 0, nothing stored), so tracfin adds
// them through RequiredSettings: nom, prenom, tel, courriel.
func requiredSettingPaths(format core.Format, values map[string]string) []core.Path {
	spec := format.Spec()
	seen := map[core.Path]bool{}
	hasValueUnder := func(group string) bool {
		for p := range values {
			if strings.HasPrefix(p, group+".") {
				return true
			}
		}
		return false
	}
	var walk func(n *core.Node, groupsPresent bool)
	walk = func(n *core.Node, groupsPresent bool) {
		if n == nil || strings.Contains(n.Template, "[]") {
			return
		}
		switch n.Kind {
		case core.NodeField:
			if groupsPresent && n.EffectiveNature() == core.NatureSetting && n.Min >= 1 {
				seen[core.Path(n.Template)] = true
			}
		case core.NodeGroup:
			present := groupsPresent && (n.Template == "" || n.Min >= 1 || hasValueUnder(n.Template))
			for _, c := range n.Children {
				walk(c, present)
			}
		}
	}
	walk(spec.Root, true)
	if r, ok := format.(core.SettingsRequirer); ok {
		for _, p := range r.RequiredSettings() {
			seen[p] = true
		}
	}
	out := make([]core.Path, 0, len(seen))
	for p := range seen {
		out = append(out, p)
	}
	slices.Sort(out)
	return out
}
