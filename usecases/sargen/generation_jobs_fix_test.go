package sargen

import (
	"testing"
	"time"

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
