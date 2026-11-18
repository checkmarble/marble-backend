package sargen

import (
	"context"
	"encoding/json"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/sargen/core"
)

// fakeGenRepo is an in-memory SarGenerationRepository. It enforces the
// partial unique index "one pending/running job per declaration" on
// CreateSarGenerationJob and SetSarGenerationJobStatus
// (models.ErrSarJobAlreadyActive). Tx has no rollback: a failure after the job
// insert leaves the job (tests of atomicity must check the call order).
type fakeGenRepo struct {
	mu        sync.Mutex
	clock     func() time.Time
	Jobs      map[uuid.UUID]models.SarGenerationJob
	Tasks     map[uuid.UUID][]models.SarGenerationTask // by job id, creation order
	Questions map[uuid.UUID]models.SarQuestion
	// FailJobCreate makes CreateSarGenerationJob fail.
	FailJobCreate error
	// JobCreates counts CreateSarGenerationJob calls that succeeded.
	JobCreates int
	seq        int
	order      map[uuid.UUID]int
}

func newFakeGenRepo(clock func() time.Time) *fakeGenRepo {
	return &fakeGenRepo{
		clock:     clock,
		Jobs:      map[uuid.UUID]models.SarGenerationJob{},
		Tasks:     map[uuid.UUID][]models.SarGenerationTask{},
		Questions: map[uuid.UUID]models.SarQuestion{},
		order:     map[uuid.UUID]int{},
	}
}

func isActiveStatus(s models.SarJobStatus) bool {
	return s == models.SarJobStatusPending || s == models.SarJobStatusRunning
}

func (r *fakeGenRepo) hasActive(declId, except uuid.UUID) bool {
	for _, j := range r.Jobs {
		if j.DeclarationId == declId && j.Id != except && isActiveStatus(j.Status) {
			return true
		}
	}
	return false
}

func (r *fakeGenRepo) CreateSarGenerationJob(_ context.Context, _ repositories.Executor, j models.SarGenerationJob) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.FailJobCreate != nil {
		return r.FailJobCreate
	}
	if isActiveStatus(j.Status) && r.hasActive(j.DeclarationId, j.Id) {
		return models.ErrSarJobAlreadyActive
	}
	now := r.clock()
	j.CreatedAt, j.UpdatedAt = now, now
	r.seq++
	r.order[j.Id] = r.seq
	r.Jobs[j.Id] = j
	r.JobCreates++
	return nil
}

func (r *fakeGenRepo) GetSarGenerationJob(_ context.Context, _ repositories.Executor, id uuid.UUID) (models.SarGenerationJob, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	j, ok := r.Jobs[id]
	if !ok {
		return models.SarGenerationJob{}, errors.Wrap(models.NotFoundError, "job")
	}
	return j, nil
}

// ListSarGenerationJobs is newest first (creation time, then insertion).
func (r *fakeGenRepo) ListSarGenerationJobs(_ context.Context, _ repositories.Executor, declId uuid.UUID) ([]models.SarGenerationJob, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := []models.SarGenerationJob{}
	for _, j := range r.Jobs {
		if j.DeclarationId == declId {
			out = append(out, j)
		}
	}
	sort.Slice(out, func(a, b int) bool {
		if !out[a].CreatedAt.Equal(out[b].CreatedAt) {
			return out[a].CreatedAt.After(out[b].CreatedAt)
		}
		return r.order[out[a].Id] > r.order[out[b].Id]
	})
	return out, nil
}

func (r *fakeGenRepo) SetSarGenerationJobStatus(_ context.Context, _ repositories.Executor, id uuid.UUID,
	status models.SarJobStatus, errMsg string,
) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	j, ok := r.Jobs[id]
	if !ok {
		return errors.Wrap(models.NotFoundError, "job")
	}
	if isActiveStatus(status) && r.hasActive(j.DeclarationId, id) {
		return models.ErrSarJobAlreadyActive
	}
	j.Status, j.Error, j.UpdatedAt = status, errMsg, r.clock()
	r.Jobs[id] = j
	return nil
}

// ResumeSarGenerationJob mirrors the SQL guard: failed, or pending/running
// untouched for longer than staleAfter on the fake clock.
func (r *fakeGenRepo) ResumeSarGenerationJob(_ context.Context, _ repositories.Executor, id uuid.UUID,
	staleAfter time.Duration,
) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	j, ok := r.Jobs[id]
	if !ok {
		return false, errors.Wrap(models.NotFoundError, "job")
	}
	stale := isActiveStatus(j.Status) && r.clock().Sub(j.UpdatedAt) > staleAfter
	if j.Status != models.SarJobStatusFailed && !stale {
		return false, nil
	}
	if r.hasActive(j.DeclarationId, id) {
		return false, models.ErrSarJobAlreadyActive
	}
	j.Status, j.Error, j.UpdatedAt = models.SarJobStatusPending, "", r.clock()
	r.Jobs[id] = j
	return true, nil
}

// FailStaleSarGenerationJobs mirrors the SQL guard on the fake clock.
func (r *fakeGenRepo) FailStaleSarGenerationJobs(_ context.Context, _ repositories.Executor, declId uuid.UUID,
	staleAfter time.Duration, errMsg string,
) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	n := 0
	for id, j := range r.Jobs {
		if j.DeclarationId == declId && isActiveStatus(j.Status) && r.clock().Sub(j.UpdatedAt) > staleAfter {
			j.Status, j.Error, j.UpdatedAt = models.SarJobStatusFailed, errMsg, r.clock()
			r.Jobs[id] = j
			n++
		}
	}
	return n, nil
}

func (r *fakeGenRepo) ListSarGenerationTaskSummaries(_ context.Context, _ repositories.Executor, jobId uuid.UUID) ([]models.SarGenerationTask, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]models.SarGenerationTask(nil), r.Tasks[jobId]...), nil
}

func (r *fakeGenRepo) ListSarQuestions(_ context.Context, _ repositories.Executor, declId uuid.UUID, status string) ([]models.SarQuestion, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := []models.SarQuestion{}
	for _, q := range r.Questions {
		if q.DeclarationId == declId && (status == "" || q.Status == status) {
			out = append(out, q)
		}
	}
	sort.Slice(out, func(a, b int) bool { return out[a].CreatedAt.Before(out[b].CreatedAt) })
	return out, nil
}

func (r *fakeGenRepo) GetSarQuestion(_ context.Context, _ repositories.Executor, id uuid.UUID) (models.SarQuestion, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	q, ok := r.Questions[id]
	if !ok {
		return models.SarQuestion{}, errors.Wrap(models.NotFoundError, "question")
	}
	return q, nil
}

func (r *fakeGenRepo) CloseSarQuestions(_ context.Context, _ repositories.Executor, ids []uuid.UUID, status string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	now := r.clock()
	for _, id := range ids {
		if q, ok := r.Questions[id]; ok {
			q.Status, q.ClosedAt = status, &now
			r.Questions[id] = q
		}
	}
	return nil
}

// fakeEnqueuer records EnqueueSarGenerationTask calls.
type fakeEnqueuer struct {
	mu    sync.Mutex
	Calls []enqueueCall
	// Err makes the next calls fail (not recorded).
	Err error
}

type enqueueCall struct{ OrgId, JobId, DeclarationId uuid.UUID }

func (e *fakeEnqueuer) EnqueueSarGenerationTask(_ context.Context, _ repositories.Transaction,
	orgId, jobId, declId uuid.UUID,
) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.Err != nil {
		return e.Err
	}
	e.Calls = append(e.Calls, enqueueCall{orgId, jobId, declId})
	return nil
}

// fakeAiAccess is an AiAccessChecker with an allow/deny switch (allowed by
// default via newTestUsecase).
type fakeAiAccess struct {
	mu      sync.Mutex
	allowed bool
	Checks  int
}

func (a *fakeAiAccess) Allow() { a.mu.Lock(); a.allowed = true; a.mu.Unlock() }
func (a *fakeAiAccess) Deny()  { a.mu.Lock(); a.allowed = false; a.mu.Unlock() }

func (a *fakeAiAccess) CheckAiAccess(_ context.Context, _ uuid.UUID) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.Checks++
	if !a.allowed {
		return errors.Wrap(models.ForbiddenError, "IA non disponible")
	}
	return nil
}

// Clock is the usecase clock of newTestUsecase (Deps.Now); f.Advance moves it.
type fakeClock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *fakeClock) Now() time.Time { c.mu.Lock(); defer c.mu.Unlock(); return c.t }

func (c *fakeClock) Advance(d time.Duration) { c.mu.Lock(); c.t = c.t.Add(d); c.mu.Unlock() }

// SeedJob inserts a job (any status, bypassing the active-job rule) created
// now; use SetJobUpdatedAt to age it.
func (f *fakes) SeedJob(t *testing.T, declId uuid.UUID, kind models.SarJobKind, status models.SarJobStatus) models.SarGenerationJob {
	t.Helper()
	now := f.Clock.Now()
	j := models.SarGenerationJob{
		Id: uuid.New(), DeclarationId: declId, Kind: kind, Status: status,
		CreatedAt: now, UpdatedAt: now,
	}
	f.Gen.mu.Lock()
	f.Gen.seq++
	f.Gen.order[j.Id] = f.Gen.seq
	f.Gen.Jobs[j.Id] = j
	f.Gen.mu.Unlock()
	return j
}

// SetJobAttempt overrides the River attempt stored on the job.
func (f *fakes) SetJobAttempt(jobId uuid.UUID, attempt int) {
	f.Gen.mu.Lock()
	defer f.Gen.mu.Unlock()
	j := f.Gen.Jobs[jobId]
	j.Attempt = attempt
	f.Gen.Jobs[jobId] = j
}

// SetJobUpdatedAt overrides updated_at (heartbeat age).
func (f *fakes) SetJobUpdatedAt(jobId uuid.UUID, at time.Time) {
	f.Gen.mu.Lock()
	defer f.Gen.mu.Unlock()
	j := f.Gen.Jobs[jobId]
	j.UpdatedAt = at
	f.Gen.Jobs[jobId] = j
}

// SeedTask adds a task to a job.
func (f *fakes) SeedTask(jobId uuid.UUID, kind, target string, status models.SarJobStatus, attempts int, errMsg string) models.SarGenerationTask {
	task := models.SarGenerationTask{
		Id: uuid.New(), JobId: jobId, Kind: kind, TargetPath: target,
		Status: status, Attempts: attempts, Error: errMsg,
	}
	f.Gen.mu.Lock()
	f.Gen.Tasks[jobId] = append(f.Gen.Tasks[jobId], task)
	f.Gen.mu.Unlock()
	return task
}

// SeedDoneGeneration inserts a done `generate` job with one done task, as left
// by a finished generation (no document change).
func (f *fakes) SeedDoneGeneration(t *testing.T, declId uuid.UUID) models.SarGenerationJob {
	t.Helper()
	j := f.SeedJob(t, declId, models.SarJobKindGenerate, models.SarJobStatusDone)
	f.SeedTask(j.Id, "fill", "", models.SarJobStatusDone, 1, "")
	return j
}

// SeedQuestion inserts an open question (kind question) with options.
func (f *fakes) SeedQuestion(t *testing.T, declId uuid.UUID, paths []core.Path, text string, opts []core.QuestionOption) models.SarQuestion {
	t.Helper()
	q := models.SarQuestion{
		Id: uuid.New(), DeclarationId: declId, Kind: models.SarQuestionKindQuestion,
		Paths: pathStrings(paths), Text: text, Status: models.SarQuestionStatusOpen,
		CreatedAt: f.Clock.Now(),
	}
	if len(opts) > 0 {
		raw, err := json.Marshal(opts)
		require.NoError(t, err)
		q.Options = raw
	}
	f.Gen.mu.Lock()
	f.Gen.Questions[q.Id] = q
	f.Gen.mu.Unlock()
	return q
}

// SeedReviewFinding inserts an open review finding (severity "important"|"mineur").
func (f *fakes) SeedReviewFinding(t *testing.T, declId uuid.UUID, paths []core.Path, severity, text string) models.SarQuestion {
	t.Helper()
	q := models.SarQuestion{
		Id: uuid.New(), DeclarationId: declId, Kind: models.SarQuestionKindReviewFinding,
		Severity: severity, Paths: pathStrings(paths), Text: text,
		Status: models.SarQuestionStatusOpen, CreatedAt: f.Clock.Now(),
	}
	f.Gen.mu.Lock()
	f.Gen.Questions[q.Id] = q
	f.Gen.mu.Unlock()
	return q
}

// JobsOf lists the jobs of a declaration (newest first).
func (f *fakes) JobsOf(declId uuid.UUID) []models.SarGenerationJob {
	jobs, _ := f.Gen.ListSarGenerationJobs(context.Background(), nil, declId)
	return jobs
}

func pathStrings(paths []core.Path) []string {
	out := make([]string, len(paths))
	for i, p := range paths {
		out[i] = string(p)
	}
	return out
}

var (
	_ SarGenerationRepository = (*fakeGenRepo)(nil)
	_ GenerationEnqueuer      = (*fakeEnqueuer)(nil)
	_ AiAccessChecker         = (*fakeAiAccess)(nil)
)
