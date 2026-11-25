package sargen

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/usecases/sargen/core"
)

// Fix wave PR 4 (J1, J3): a failed generation is resumed, not replaced, and
// a stale active job does not block the declaration writers.

// ---- J1: relaunch after a failed generate job ----

// PR 6 B1 / PR 7 B1 (verification-jobs items 1 and 3): after a failed
// generation, a second StartGeneration must resume the same job. A new job
// would plan the placement again and create the persons and transactions a
// second time, since the done tasks belong to the job that failed.
func TestStartGeneration_AfterFailedJobResumesIt(t *testing.T) {
	uc, f, declId := genReady(t, false)
	j1, err := uc.StartGeneration(f.Ctx, declId)
	require.NoError(t, err)
	f.SeedTask(j1.Id, "fill", "organisme.libelle", models.SarJobStatusDone, 1, "")
	genMarkJob(t, f, j1.Id, models.SarJobStatusFailed, "fournisseur indisponible")

	j2, err := uc.StartGeneration(f.Ctx, declId)
	require.NoError(t, err)
	assert.Equal(t, j1.Id, j2.Id, "the failed generation is resumed under its own id")
	assert.Len(t, f.JobsOf(declId), 1, "no second job must be created")
	assert.Equal(t, 1, f.Gen.JobCreates)
	assert.Equal(t, models.SarJobStatusPending, f.Gen.Jobs[j1.Id].Status)
	require.Len(t, f.Enqueuer.Calls, 2, "the resumed job is enqueued again")
	assert.Equal(t, enqueueCall{OrgId: f.OrgId, JobId: j1.Id, DeclarationId: declId}, f.Enqueuer.Calls[1])
	assert.Len(t, f.Gen.Tasks[j1.Id], 1, "done tasks are kept as they are, not replayed by a new job")
}

// PR 4 I2 / PR 7 B1: once a task of a generate job is done, the selection is
// frozen, even if that job failed afterwards (the placement results exist).
func TestUpdateSelection_FrozenAfterFailedJobWithDoneTask(t *testing.T) {
	uc, f, declId := genReady(t, false)
	j1, err := uc.StartGeneration(f.Ctx, declId)
	require.NoError(t, err)
	f.SeedTask(j1.Id, "fill", "organisme.libelle", models.SarJobStatusDone, 1, "")
	genMarkJob(t, f, j1.Id, models.SarJobStatusFailed, "fournisseur indisponible")

	f.Decisions.Add(f.OrgId, "transactions", map[string]any{"object_id": "t2", "amount": 7}, time.Now())
	version := f.Repo.Declarations[declId].Version

	_, err = uc.UpdateSelection(f.Ctx, declId, version, []SelectionAdd{{Ref: selRef("transactions", "t2")}}, nil)
	require.ErrorIs(t, err, models.ConflictError)
	assert.Len(t, f.Selection.Rows, 1, "the selection must not change once a generated task is done")
}

// ---- J3: stale active job vs declaration writers ----

// PR 4 I3 (verification-jobs item 4): an active job with no update for more
// than 2 x the job timeout is stale. It must not keep the declaration locked
// forever: a writer fails the stale job and goes on, even when the AI is not
// available (the analyst then has no way to resume the job).
func TestStaleJobDoesNotBlockWriters(t *testing.T) {
	t.Run("manual edit", func(t *testing.T) {
		uc, f, declId := genReady(t, true)
		job := f.SeedJob(t, declId, models.SarJobKindGenerate, models.SarJobStatusRunning)
		f.Clock.Advance(2 * time.Hour)
		f.AI.Deny()

		st, err := uc.UpdateFields(f.Ctx, declId, f.Repo.Declarations[declId].Version,
			[]core.Fill{{Path: "organisme.libelle", Value: "Autre banque"}})
		require.NoError(t, err)
		assert.Equal(t, models.SarJobStatusFailed, f.Gen.Jobs[job.Id].Status, "the stale job is failed by the write")
		assert.Equal(t, f.Repo.Declarations[declId].Version, st.Declaration.Version)
	})

	t.Run("export", func(t *testing.T) {
		uc, f, declId := genReady(t, true)
		job := f.SeedJob(t, declId, models.SarJobKindGenerate, models.SarJobStatusRunning)
		f.Clock.Advance(2 * time.Hour)
		f.AI.Deny()

		_, err := uc.Export(f.Ctx, declId, f.Repo.Declarations[declId].Version)
		require.NoError(t, err)
		assert.Equal(t, models.SarJobStatusFailed, f.Gen.Jobs[job.Id].Status, "the stale job is failed by the export")
	})
}

// ---- Round 1: selection of a failed job with no done task ----

// A failed generation with no done task keeps the selection open; its
// tasks (old snapshots) are dropped so that the resume replans
// from the new selection and runs nothing stale.
func TestUpdateSelection_FailedJobWithoutDoneTaskStaysOpenAndDropsUnfinishedTasks(t *testing.T) {
	uc, f, declId := genReady(t, false)
	j1, err := uc.StartGeneration(f.Ctx, declId)
	require.NoError(t, err)
	f.SeedTask(j1.Id, "fill", "organisme.libelle", models.SarJobStatusPending, 0, "")
	f.SeedTask(j1.Id, "fill", "organisme.adresse", models.SarJobStatusFailed, 1, "boom")
	f.SeedTask(j1.Id, "fill", "declarant", models.SarJobStatusRunning, 1, "")
	genMarkJob(t, f, j1.Id, models.SarJobStatusFailed, "fournisseur indisponible")

	f.Decisions.Add(f.OrgId, "transactions", map[string]any{"object_id": "t2", "amount": 7}, time.Now())
	version := f.Repo.Declarations[declId].Version
	_, err = uc.UpdateSelection(f.Ctx, declId, version, []SelectionAdd{{Ref: selRef("transactions", "t2")}}, nil)
	require.NoError(t, err)
	assert.Len(t, f.Selection.Rows, 2, "the selection changed")
	assert.Empty(t, f.Gen.Tasks[j1.Id], "unfinished tasks of the failed job are dropped")

	resumed, err := uc.StartGeneration(f.Ctx, declId)
	require.NoError(t, err)
	assert.Equal(t, j1.Id, resumed.Id)
	assert.Empty(t, f.Gen.Tasks[j1.Id], "nothing stale left for the resume: the orchestrator plans from the new selection")
}

// A done task that wrote nothing (the understanding step) does not freeze the
// selection: the job failed at the placement, the analyst changes the selection
// and the resume replans everything, the understanding included.
func TestUpdateSelection_ReadOnlyDoneTaskDoesNotFreeze(t *testing.T) {
	uc, f, declId := genReady(t, false)
	j1, err := uc.StartGeneration(f.Ctx, declId)
	require.NoError(t, err)
	f.SeedReadOnlyDoneTask(j1.Id, "understanding")
	f.SeedTask(j1.Id, "placement", "personnes", models.SarJobStatusFailed, 1, "boom")
	genMarkJob(t, f, j1.Id, models.SarJobStatusFailed, "fournisseur indisponible")

	f.Decisions.Add(f.OrgId, "transactions", map[string]any{"object_id": "t2", "amount": 7}, time.Now())
	version := f.Repo.Declarations[declId].Version
	_, err = uc.UpdateSelection(f.Ctx, declId, version, []SelectionAdd{{Ref: selRef("transactions", "t2")}}, nil)
	require.NoError(t, err)
	assert.Len(t, f.Selection.Rows, 2, "the selection changed")
	assert.Empty(t, f.Gen.Tasks[j1.Id], "every task is dropped, the done read-only one included")

	resumed, err := uc.StartGeneration(f.Ctx, declId)
	require.NoError(t, err)
	assert.Equal(t, j1.Id, resumed.Id)
	assert.Empty(t, f.Gen.Tasks[j1.Id], "the orchestrator replans from the understanding")
}

// Once a task wrote to the document the selection is frozen, whatever the
// read-only tasks around it.
func TestUpdateSelection_FrozenWhenAWritingTaskIsDone(t *testing.T) {
	uc, f, declId := genReady(t, false)
	j1, err := uc.StartGeneration(f.Ctx, declId)
	require.NoError(t, err)
	f.SeedReadOnlyDoneTask(j1.Id, "understanding")
	wrote := f.SeedTask(j1.Id, "placement", "personnes", models.SarJobStatusDone, 1, "")
	f.Gen.mu.Lock()
	f.Gen.Tasks[j1.Id][1].Output = []byte(`{"applied":["personnes.pp[0].nom"],"wrote":true}`)
	f.Gen.mu.Unlock()
	_ = wrote
	genMarkJob(t, f, j1.Id, models.SarJobStatusFailed, "fournisseur indisponible")

	f.Decisions.Add(f.OrgId, "transactions", map[string]any{"object_id": "t2", "amount": 7}, time.Now())
	version := f.Repo.Declarations[declId].Version
	_, err = uc.UpdateSelection(f.Ctx, declId, version, []SelectionAdd{{Ref: selRef("transactions", "t2")}}, nil)
	require.ErrorIs(t, err, ErrSelectionFrozen)
	assert.Len(t, f.Gen.Tasks[j1.Id], 2, "nothing deleted")
}

// A review job never freezes the selection, even with done tasks.
func TestUpdateSelection_ReviewJobDoesNotFreeze(t *testing.T) {
	uc, f, declId := genReady(t, false)
	rv := f.SeedJob(t, declId, models.SarJobKindReview, models.SarJobStatusFailed)
	f.SeedTask(rv.Id, "review", "", models.SarJobStatusDone, 1, "")

	f.Decisions.Add(f.OrgId, "transactions", map[string]any{"object_id": "t2", "amount": 7}, time.Now())
	version := f.Repo.Declarations[declId].Version
	_, err := uc.UpdateSelection(f.Ctx, declId, version, []SelectionAdd{{Ref: selRef("transactions", "t2")}}, nil)
	require.NoError(t, err)
	assert.Len(t, f.Gen.Tasks[rv.Id], 1, "review job tasks are untouched")
}

// ---- Final review M2: ResumeJob applies the preconditions of StartGeneration ----

func TestResumeJob_GenerateAppliesStartPreconditions(t *testing.T) {
	failedJob := func(t *testing.T) (*Usecase, *fakes, uuid.UUID, models.SarGenerationJob) {
		uc, f, declId := genReady(t, false)
		job := f.SeedJob(t, declId, models.SarJobKindGenerate, models.SarJobStatusFailed)
		return uc, f, declId, job
	}
	assertRefused := func(t *testing.T, f *fakes, jobId uuid.UUID, err error, contains string) {
		t.Helper()
		require.ErrorIs(t, err, models.BadParameterError)
		if contains != "" {
			require.Contains(t, errorAndIssuePaths(err), contains)
		}
		assert.Equal(t, models.SarJobStatusFailed, f.Gen.Jobs[jobId].Status, "the job stays failed")
		assert.Empty(t, f.Enqueuer.Calls, "nothing is enqueued")
	}

	t.Run("empty selection", func(t *testing.T) {
		uc, f, _, job := failedJob(t)
		f.Selection.Rows = nil
		_, err := uc.ResumeJob(f.Ctx, job.Id)
		assertRefused(t, f, job.Id, err, "sélection")
	})
	t.Run("empty analysis", func(t *testing.T) {
		uc, f, declId, job := failedJob(t)
		d := f.Repo.Declarations[declId]
		d.Analysis = ""
		f.Repo.Declarations[declId] = d
		_, err := uc.ResumeJob(f.Ctx, job.Id)
		assertRefused(t, f, job.Id, err, "analyse")
	})
	t.Run("incomplete settings", func(t *testing.T) {
		uc, f, _, job := failedJob(t)
		incomplete := genCompleteSettings()
		delete(incomplete, "declarant.coordonnees.tel")
		genSetSettings(f, incomplete)
		_, err := uc.ResumeJob(f.Ctx, job.Id)
		assertRefused(t, f, job.Id, err, "declarant.coordonnees.tel")
	})
	t.Run("a review job is not subject to them", func(t *testing.T) {
		uc, f, declId := genReady(t, false)
		job := f.SeedJob(t, declId, models.SarJobKindReview, models.SarJobStatusFailed)
		f.Selection.Rows = nil
		_, err := uc.ResumeJob(f.Ctx, job.Id)
		require.NoError(t, err)
	})
}

// The questions raised by the tasks of a failed job are dismissed with those
// tasks when a selection change deletes them; the other questions stay open.
func TestUpdateSelection_DismissesTheQuestionsOfTheDroppedTasks(t *testing.T) {
	uc, f, declId := genReady(t, false)
	j1, err := uc.StartGeneration(f.Ctx, declId)
	require.NoError(t, err)
	task := f.SeedTask(j1.Id, "placement", "personnes", models.SarJobStatusFailed, 1, "boom")
	genMarkJob(t, f, j1.Id, models.SarJobStatusFailed, "fournisseur indisponible")
	ofTask := f.SeedQuestion(t, declId, []core.Path{"organisme.libelle"}, "Personne non déclarée ?", nil)
	ofTask.TaskId = &task.Id
	f.Gen.Questions[ofTask.Id] = ofTask
	foreign := f.SeedReviewFinding(t, declId, []core.Path{"organisme.adresse"}, "mineur", "Autre")

	f.Decisions.Add(f.OrgId, "transactions", map[string]any{"object_id": "t2", "amount": 7}, time.Now())
	version := f.Repo.Declarations[declId].Version
	_, err = uc.UpdateSelection(f.Ctx, declId, version, []SelectionAdd{{Ref: selRef("transactions", "t2")}}, nil)
	require.NoError(t, err)
	assert.Empty(t, f.Gen.Tasks[j1.Id])
	assert.Equal(t, models.SarQuestionStatusDismissed, f.Gen.Questions[ofTask.Id].Status)
	assert.Equal(t, models.SarQuestionStatusOpen, f.Gen.Questions[foreign.Id].Status)
}
