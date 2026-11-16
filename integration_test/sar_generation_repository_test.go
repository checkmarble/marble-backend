package integration

import (
	"testing"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

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
	require.NoError(t, repo.SetSarGenerationJobStatus(ctx, exec, first.Id, models.SarJobStatusRunning, ""))
	err = repo.CreateSarGenerationJob(ctx, exec, second)
	require.Error(t, err)
	require.True(t, errors.Is(err, models.ErrSarJobAlreadyActive))
	require.True(t, errors.Is(err, models.ConflictError))

	// Once the first job is done, a new job is accepted.
	require.NoError(t, repo.SetSarGenerationJobStatus(ctx, exec, first.Id, models.SarJobStatusDone, ""))
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

	require.NoError(t, repo.SetSarGenerationJobStatus(ctx, exec, first.Id, models.SarJobStatusFailed, "boom"))
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

	claimed, err := repo.ClaimSarGenerationTask(ctx, exec, task.Id, time.Now())
	require.NoError(t, err)
	require.True(t, claimed)

	// Simulate a worker that died long ago while the task was running.
	_, err = pgPool.Exec(ctx, "UPDATE sar_generation_tasks SET started_at = $1 WHERE id = $2",
		time.Now().Add(-2*time.Hour), task.Id)
	require.NoError(t, err)

	claimed, err = repo.ClaimSarGenerationTask(ctx, exec, task.Id, time.Now().Add(-time.Hour))
	require.NoError(t, err)
	require.True(t, claimed)

	tasks, err := repo.ListSarGenerationTasks(ctx, exec, job.Id)
	require.NoError(t, err)
	require.Len(t, tasks, 1)
	require.Equal(t, models.SarJobStatusRunning, tasks[0].Status)
	require.Equal(t, 2, tasks[0].Attempts)
}

func TestClaimTask_FreshRunningIsNotClaimable(t *testing.T) {
	ctx := t.Context()
	repo, _, job := sarGenTestSetup(t)
	exec := testUsecases.NewExecutorFactory().NewExecutor()

	task := newSarGenTestTask(job.Id, "part.a")
	require.NoError(t, repo.CreateSarGenerationTasks(ctx, exec, []models.SarGenerationTask{task}))

	claimed, err := repo.ClaimSarGenerationTask(ctx, exec, task.Id, time.Now())
	require.NoError(t, err)
	require.True(t, claimed)

	// The task just started: a claim whose stale threshold is one hour ago must be refused.
	claimed, err = repo.ClaimSarGenerationTask(ctx, exec, task.Id, time.Now().Add(-time.Hour))
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

	claimed, err := repo.ClaimSarGenerationTask(ctx, exec, task.Id, time.Now())
	require.NoError(t, err)
	require.True(t, claimed)

	output := []byte(`{"fills":[{"path":"part.a.nom","value":"Dupont"}]}`)
	trace := []byte(`[{"role":"assistant","content":"reasoning"}]`)
	require.NoError(t, repo.CompleteSarGenerationTask(ctx, exec, task.Id, output, trace))

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
}

func TestFailTask_StoresErrorAndClaimableAgain(t *testing.T) {
	ctx := t.Context()
	repo, _, job := sarGenTestSetup(t)
	exec := testUsecases.NewExecutorFactory().NewExecutor()

	task := newSarGenTestTask(job.Id, "part.a")
	require.NoError(t, repo.CreateSarGenerationTasks(ctx, exec, []models.SarGenerationTask{task}))

	claimed, err := repo.ClaimSarGenerationTask(ctx, exec, task.Id, time.Now())
	require.NoError(t, err)
	require.True(t, claimed)

	trace := []byte(`[{"role":"assistant","content":"failed attempt"}]`)
	require.NoError(t, repo.FailSarGenerationTask(ctx, exec, task.Id, "llm timeout", trace))

	tasks, err := repo.ListSarGenerationTasks(ctx, exec, job.Id)
	require.NoError(t, err)
	require.Len(t, tasks, 1)
	require.Equal(t, models.SarJobStatusFailed, tasks[0].Status)
	require.Equal(t, "llm timeout", tasks[0].Error)

	gotTrace, err := repo.GetSarGenerationTaskTrace(ctx, exec, task.Id)
	require.NoError(t, err)
	require.JSONEq(t, string(trace), string(gotTrace))

	// A failed task can be claimed again, whatever the stale threshold.
	claimed, err = repo.ClaimSarGenerationTask(ctx, exec, task.Id, time.Now().Add(-time.Hour))
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
