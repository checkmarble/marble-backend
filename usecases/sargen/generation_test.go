package sargen

import (
	"bytes"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/usecases/sargen/core"
)

// Task 11 bis: start / status / resume / review start, job lock.
// Shared helpers for generation_test.go and questions_test.go.

// genCompleteSettings returns organization settings that fill every required
// TRACFIN setting (values taken from testdata/minimal.json).
func genCompleteSettings() map[string]string {
	return map[string]string{
		"organisme.libelle":              "Banque Exemple SA",
		"organisme.numIdPro":             "123456789",
		"organisme.profession":           "TPI1",
		"organisme.adresse.codePostal":   "75002",
		"organisme.adresse.nomVoie":      "Rue de la Paix",
		"organisme.adresse.pays":         "FR",
		"organisme.adresse.tel":          "+33 1 40 00 00 00",
		"organisme.adresse.ville":        "Paris",
		"declarant.coordonnees.nom":      "Dupont",
		"declarant.coordonnees.prenom":   "Jean",
		"declarant.coordonnees.tel":      "+33 1 23 45 67 89",
		"declarant.coordonnees.courriel": "jean.dupont@banque.fr",
	}
}

// genSetSettings stores the organization settings of the TRACFIN family.
func genSetSettings(f *fakes, values map[string]string) {
	f.Repo.Settings[settingsKey(f.OrgId, tracfinFamily)] = models.SarOrgFormatSettings{
		OrganizationId: f.OrgId,
		FormatFamily:   tracfinFamily,
		Values:         values,
	}
}

// genAnalyse sets a non-empty analysis and returns the new version.
func genAnalyse(t *testing.T, uc *Usecase, f *fakes, declId uuid.UUID) int {
	t.Helper()
	st, err := uc.UpdateAnalysis(f.Ctx, declId, f.Repo.Declarations[declId].Version, "Soupçon de test", "")
	require.NoError(t, err)
	return st.Declaration.Version
}

// genSelect adds one transaction (trigger object from a decision) to the
// selection and returns the new version.
func genSelect(t *testing.T, uc *Usecase, f *fakes, declId uuid.UUID, objectId string) int {
	t.Helper()
	f.Decisions.Add(f.OrgId, "transactions", map[string]any{"object_id": objectId, "amount": 42}, time.Now())
	st, err := uc.UpdateSelection(f.Ctx, declId, f.Repo.Declarations[declId].Version,
		[]SelectionAdd{{Ref: selRef("transactions", objectId)}}, nil)
	require.NoError(t, err)
	return st.Declaration.Version
}

// genReady returns a TRACFIN declaration that may start a generation: complete
// organization settings, non-empty analysis and selection. With minimalDocument
// the document is the complete minimal TRACFIN document (testdata), otherwise
// the fresh document.
func genReady(t *testing.T, minimalDocument bool) (*Usecase, *fakes, uuid.UUID) {
	t.Helper()
	uc, f := newTestUsecase(t)
	genSetSettings(f, genCompleteSettings())
	var declId uuid.UUID
	if minimalDocument {
		declId = seedValidDeclaration(t, uc, f).Id
	} else {
		declId = createFresh(t, uc, f).Declaration.Id
	}
	genAnalyse(t, uc, f, declId)
	genSelect(t, uc, f, declId, "t1")
	return uc, f, declId
}

// genEditDoc edits the stored document of a declaration directly (test setup).
func genEditDoc(t *testing.T, f *fakes, declId uuid.UUID, edit func(doc *core.Document)) {
	t.Helper()
	d := f.Repo.Declarations[declId]
	doc := decodeDoc(t, d.Document)
	edit(doc)
	raw, err := json.Marshal(doc)
	require.NoError(t, err)
	d.Document = raw
	f.Repo.Declarations[declId] = d
}

// genMarkJob moves a job to a status without going through the usecase.
func genMarkJob(t *testing.T, f *fakes, jobId uuid.UUID, status models.SarJobStatus, errMsg string) {
	t.Helper()
	require.NoError(t, f.Gen.SetSarGenerationJobStatus(f.Ctx, nil, jobId, status, errMsg))
}

// ---- GetJob ----

func TestGetJob_ReportsProgressAndLastError(t *testing.T) {
	uc, f, declId := genReady(t, false)

	t.Run("job error wins over task errors", func(t *testing.T) {
		job := f.SeedJob(t, declId, models.SarJobKindGenerate, models.SarJobStatusRunning)
		f.SeedTask(job.Id, "fill", "organisme.libelle", models.SarJobStatusDone, 1, "")
		f.SeedTask(job.Id, "fill", "organisme.numIdPro", models.SarJobStatusDone, 1, "")
		f.SeedTask(job.Id, "fill", "operations.montantTotal", models.SarJobStatusFailed, 3, "erreur de tâche")
		f.SeedTask(job.Id, "fill", "personnes.pp", models.SarJobStatusPending, 0, "")
		genMarkJob(t, f, job.Id, models.SarJobStatusFailed, "délai dépassé")
		f.SetJobAttempt(job.Id, 3)

		st, err := uc.GetJob(f.Ctx, job.Id)
		require.NoError(t, err)
		assert.Equal(t, job.Id, st.Job.Id)
		assert.Equal(t, models.SarJobStatusFailed, st.Job.Status)
		assert.Equal(t, 2, st.Done, "done tasks")
		assert.Equal(t, 4, st.Total, "all tasks")
		assert.Len(t, st.Tasks, 4)
		assert.Equal(t, "délai dépassé", st.LastError)
		assert.Equal(t, 3, st.Attempt, "River attempt stored on the job")
		assert.Equal(t, models.SarGenerationMaxAttempts, st.MaxAttempts)
	})

	t.Run("without job error the last failed task error is reported", func(t *testing.T) {
		job := f.SeedJob(t, declId, models.SarJobKindGenerate, models.SarJobStatusRunning)
		f.SeedTask(job.Id, "fill", "operations.montantTotal", models.SarJobStatusFailed, 2, "erreur de tâche")
		f.SetJobAttempt(job.Id, 2)

		st, err := uc.GetJob(f.Ctx, job.Id)
		require.NoError(t, err)
		assert.Equal(t, 0, st.Done)
		assert.Equal(t, 1, st.Total)
		assert.Equal(t, "erreur de tâche", st.LastError)
		assert.Equal(t, 2, st.Attempt)
	})
}

func TestGetJob_OtherOrgForbidden(t *testing.T) {
	uc, f := newTestUsecase(t)
	foreign := models.SarDeclaration{
		Id:             uuid.New(),
		OrganizationId: uuid.New(), // another organization
		CaseId:         f.CaseId,
		FormatKey:      tracfinFormatKey,
		ReportType:     tracfinReportType,
		Version:        1,
		Document:       []byte(`{"values":{},"items":{},"item_sources":{},"next_key":{}}`),
	}
	require.NoError(t, f.Repo.CreateSarDeclaration(f.Ctx, nil, foreign))
	job := f.SeedJob(t, foreign.Id, models.SarJobKindGenerate, models.SarJobStatusRunning)

	_, err := uc.GetJob(f.Ctx, job.Id)
	require.ErrorIs(t, err, models.ForbiddenError)
}

// ---- StartGeneration ----

func TestStartGeneration_RefusedWhenAlreadyGenerated(t *testing.T) {
	uc, f, declId := genReady(t, false)
	f.SeedDoneGeneration(t, declId)

	_, err := uc.StartGeneration(f.Ctx, declId)
	require.ErrorIs(t, err, ErrAlreadyGenerated)
	assert.ErrorIs(t, err, models.ConflictError)
	assert.Zero(t, f.Gen.JobCreates, "no job must be created")
	assert.Empty(t, f.Enqueuer.Calls)
}

func TestStartGeneration_RefusedWithoutAiAccess(t *testing.T) {
	uc, f, declId := genReady(t, false)
	f.AI.Deny()

	_, err := uc.StartGeneration(f.Ctx, declId)
	require.ErrorIs(t, err, models.ForbiddenError)
	assert.Zero(t, f.Gen.JobCreates, "no job must be created")
	assert.Empty(t, f.Enqueuer.Calls)
}

func TestStartGeneration_RefusedWithoutAnalysisOrSelection(t *testing.T) {
	t.Run("empty analysis", func(t *testing.T) {
		uc, f := newTestUsecase(t)
		genSetSettings(f, genCompleteSettings())
		declId := createFresh(t, uc, f).Declaration.Id
		genSelect(t, uc, f, declId, "t1")

		_, err := uc.StartGeneration(f.Ctx, declId)
		require.ErrorIs(t, err, models.BadParameterError)
		assert.Zero(t, f.Gen.JobCreates)
		assert.Empty(t, f.Enqueuer.Calls)
	})

	t.Run("empty selection", func(t *testing.T) {
		uc, f := newTestUsecase(t)
		genSetSettings(f, genCompleteSettings())
		declId := createFresh(t, uc, f).Declaration.Id
		genAnalyse(t, uc, f, declId)

		_, err := uc.StartGeneration(f.Ctx, declId)
		require.ErrorIs(t, err, models.BadParameterError)
		assert.Zero(t, f.Gen.JobCreates)
		assert.Empty(t, f.Enqueuer.Calls)
	})
}

func TestStartGeneration_RefusedWhenOrgSettingsIncomplete(t *testing.T) {
	uc, f, declId := genReady(t, false)
	incomplete := genCompleteSettings()
	delete(incomplete, "declarant.coordonnees.tel")
	genSetSettings(f, incomplete)

	_, err := uc.StartGeneration(f.Ctx, declId)
	require.ErrorIs(t, err, models.BadParameterError)
	require.ErrorContains(t, err, "declarant.coordonnees.tel")
	assert.Empty(t, f.JobsOf(declId), "no job must be created")
	assert.Zero(t, f.Gen.JobCreates)
	assert.Empty(t, f.Enqueuer.Calls, "nothing must be enqueued")
}

func TestStartGeneration_ReappliesOrgSettingsFilledAfterCreation(t *testing.T) {
	uc, f := newTestUsecase(t)
	declId := createFresh(t, uc, f).Declaration.Id // created without organization settings
	genAnalyse(t, uc, f, declId)
	genSelect(t, uc, f, declId, "t1")
	before := f.Repo.Declarations[declId].Version

	genSetSettings(f, genCompleteSettings()) // filled after the declaration was created

	job, err := uc.StartGeneration(f.Ctx, declId)
	require.NoError(t, err)
	assert.Equal(t, models.SarJobKindGenerate, job.Kind)

	d := f.Repo.Declarations[declId]
	assert.Greater(t, d.Version, before, "a changed document is saved with a version bump")
	doc := decodeDoc(t, d.Document)
	nom, ok := doc.Values["declarant.coordonnees.nom"]
	require.True(t, ok, "the organization setting must reach the document at generation start")
	assert.Equal(t, "Dupont", nom.Value)
	assert.Equal(t, core.OriginOrgSetting, nom.Origin)
	libelle, ok := doc.Values["organisme.libelle"]
	require.True(t, ok)
	assert.Equal(t, "Banque Exemple SA", libelle.Value)
	assert.Equal(t, core.OriginOrgSetting, libelle.Origin)
}

func TestStartGeneration_CreatesJobAndEnqueuesOnce(t *testing.T) {
	uc, f, declId := genReady(t, false)

	job, err := uc.StartGeneration(f.Ctx, declId)
	require.NoError(t, err)
	assert.Equal(t, models.SarJobKindGenerate, job.Kind)
	assert.Equal(t, models.SarJobStatusPending, job.Status)
	assert.Equal(t, declId, job.DeclarationId)
	require.Len(t, f.Enqueuer.Calls, 1)
	assert.Equal(t, enqueueCall{OrgId: f.OrgId, JobId: job.Id, DeclarationId: declId}, f.Enqueuer.Calls[0])

	_, err = uc.StartGeneration(f.Ctx, declId)
	require.ErrorIs(t, err, models.ErrSarJobAlreadyActive)
	assert.ErrorIs(t, err, models.ConflictError)
	assert.Len(t, f.Enqueuer.Calls, 1, "a second start must not enqueue again")
	assert.Equal(t, 1, f.Gen.JobCreates)
}

// ---- ResumeJob ----

func TestResumeJob_OnlyFailedOrStale(t *testing.T) {
	t.Run("pending is refused", func(t *testing.T) {
		uc, f, declId := genReady(t, false)
		job := f.SeedJob(t, declId, models.SarJobKindGenerate, models.SarJobStatusPending)

		_, err := uc.ResumeJob(f.Ctx, job.Id)
		require.ErrorIs(t, err, models.ConflictError)
		assert.Equal(t, models.SarJobStatusPending, f.Gen.Jobs[job.Id].Status)
		assert.Empty(t, f.Enqueuer.Calls)
	})

	t.Run("failed goes back to pending and is enqueued", func(t *testing.T) {
		uc, f, declId := genReady(t, false)
		job := f.SeedJob(t, declId, models.SarJobKindGenerate, models.SarJobStatusFailed)

		resumed, err := uc.ResumeJob(f.Ctx, job.Id)
		require.NoError(t, err)
		assert.Equal(t, models.SarJobStatusPending, resumed.Status)
		assert.Equal(t, models.SarJobStatusPending, f.Gen.Jobs[job.Id].Status)
		require.Len(t, f.Enqueuer.Calls, 1)
		assert.Equal(t, job.Id, f.Enqueuer.Calls[0].JobId)
	})

	t.Run("running is resumed only after twice the job timeout", func(t *testing.T) {
		uc, f, declId := genReady(t, false)
		job := f.SeedJob(t, declId, models.SarJobKindGenerate, models.SarJobStatusRunning)

		_, err := uc.ResumeJob(f.Ctx, job.Id)
		require.ErrorIs(t, err, models.ConflictError, "a fresh running job must not be resumed")
		f.Clock.Advance(59 * time.Minute)
		_, err = uc.ResumeJob(f.Ctx, job.Id)
		require.ErrorIs(t, err, models.ConflictError, "under 2 x 30 min the job is not stale")
		assert.Empty(t, f.Enqueuer.Calls)

		f.Clock.Advance(2 * time.Minute) // 61 min since the last update
		resumed, err := uc.ResumeJob(f.Ctx, job.Id)
		require.NoError(t, err)
		assert.Equal(t, models.SarJobStatusPending, resumed.Status)
		require.Len(t, f.Enqueuer.Calls, 1)
		assert.Equal(t, job.Id, f.Enqueuer.Calls[0].JobId)
	})
}

// ---- StartReview ----

// A job whose River job was lost (never picked up, still pending) must not
// lock the declaration forever: after 2 x the job timeout it is reported
// failed and can be resumed.
func TestResumeJob_LostPendingJobAfterStartGeneration(t *testing.T) {
	uc, f, declId := genReady(t, false)
	job, err := uc.StartGeneration(f.Ctx, declId)
	require.NoError(t, err)
	require.Equal(t, models.SarJobStatusPending, job.Status)
	require.Len(t, f.Enqueuer.Calls, 1)

	// No worker ever runs it.
	_, err = uc.ResumeJob(f.Ctx, job.Id)
	require.ErrorIs(t, err, models.ConflictError, "a fresh pending job is not stale")

	f.Clock.Advance(61 * time.Minute)
	st, err := uc.GetJob(f.Ctx, job.Id)
	require.NoError(t, err)
	assert.Equal(t, models.SarJobStatusFailed, st.Job.Status)
	// Starting again fails the stale job (database clock) and resumes it.
	again, err := uc.StartGeneration(f.Ctx, declId)
	require.NoError(t, err)
	assert.Equal(t, job.Id, again.Id)
	assert.Equal(t, models.SarJobStatusPending, again.Status)
	assert.Equal(t, 1, f.Gen.JobCreates)
	require.Len(t, f.Enqueuer.Calls, 2)
	assert.Equal(t, job.Id, f.Enqueuer.Calls[1].JobId)
}

func TestStartReview_RefusedBeforeFirstGeneration(t *testing.T) {
	uc, f, declId := genReady(t, false)

	_, err := uc.StartReview(f.Ctx, declId)
	require.ErrorIs(t, err, models.ConflictError)
	assert.Zero(t, f.Gen.JobCreates)
	assert.Empty(t, f.Enqueuer.Calls)
}

func TestStartReview_AllowedRepeatedlyAndNeverChangesDocument(t *testing.T) {
	uc, f, declId := genReady(t, true)
	f.SeedDoneGeneration(t, declId)
	version := f.Repo.Declarations[declId].Version
	docBefore := append([]byte(nil), f.Repo.Declarations[declId].Document...)

	first, err := uc.StartReview(f.Ctx, declId)
	require.NoError(t, err)
	assert.Equal(t, models.SarJobKindReview, first.Kind)
	genMarkJob(t, f, first.Id, models.SarJobStatusDone, "")

	second, err := uc.StartReview(f.Ctx, declId)
	require.NoError(t, err)
	assert.Equal(t, models.SarJobKindReview, second.Kind)
	assert.NotEqual(t, first.Id, second.Id)
	require.Len(t, f.Enqueuer.Calls, 2)

	d := f.Repo.Declarations[declId]
	assert.Equal(t, version, d.Version, "a review never bumps the document version")
	assert.True(t, bytes.Equal(docBefore, d.Document), "a review never changes the document")
}

// ---- Job lock ----

func TestUpdateFields_RefusedDuringJob(t *testing.T) {
	uc, f, declId := genReady(t, true)
	f.SeedJob(t, declId, models.SarJobKindGenerate, models.SarJobStatusRunning)
	version := f.Repo.Declarations[declId].Version
	docBefore := append([]byte(nil), f.Repo.Declarations[declId].Document...)

	_, err := uc.UpdateFields(f.Ctx, declId, version, []core.Fill{{Path: "organisme.libelle", Value: "Autre banque"}})
	require.ErrorIs(t, err, ErrGenerationInProgress)
	assert.ErrorIs(t, err, models.ConflictError)

	d := f.Repo.Declarations[declId]
	assert.Equal(t, version, d.Version)
	assert.True(t, bytes.Equal(docBefore, d.Document))
}

func TestUpdateSelection_RefusedAfterGeneration(t *testing.T) {
	uc, f, declId := genReady(t, false)
	f.SeedDoneGeneration(t, declId)
	f.Decisions.Add(f.OrgId, "transactions", map[string]any{"object_id": "t2", "amount": 7}, time.Now())
	version := f.Repo.Declarations[declId].Version

	_, err := uc.UpdateSelection(f.Ctx, declId, version, []SelectionAdd{{Ref: selRef("transactions", "t2")}}, nil)
	require.ErrorIs(t, err, ErrSelectionFrozen)
	assert.ErrorIs(t, err, models.ConflictError)
	assert.Len(t, f.Selection.Rows, 1, "the selection must not change after generation")
}

type lockWriter struct {
	name string
	call func(uc *Usecase, f *fakes, declId uuid.UUID, version int) error
}

// TestLockDuringJob_AllWriters: every writer of the declaration is refused with
// 409 while a job is pending or running.
func TestLockDuringJob_AllWriters(t *testing.T) {
	writers := []lockWriter{
		{"AddItem", func(uc *Usecase, f *fakes, id uuid.UUID, v int) error {
			_, _, err := uc.AddItem(f.Ctx, id, v, "personnes.pp")
			return err
		}},
		{"RemoveItem", func(uc *Usecase, f *fakes, id uuid.UUID, v int) error {
			_, err := uc.RemoveItem(f.Ctx, id, v, "personnes.pp[pp-1]")
			return err
		}},
		{"MarkReviewed", func(uc *Usecase, f *fakes, id uuid.UUID, v int) error {
			_, err := uc.MarkReviewed(f.Ctx, id, v, []core.Path{"organisme.libelle"})
			return err
		}},
		{"UpdateFields", func(uc *Usecase, f *fakes, id uuid.UUID, v int) error {
			_, err := uc.UpdateFields(f.Ctx, id, v, []core.Fill{{Path: "organisme.libelle", Value: "Autre"}})
			return err
		}},
		{"UpdateSelection", func(uc *Usecase, f *fakes, id uuid.UUID, v int) error {
			_, err := uc.UpdateSelection(f.Ctx, id, v, []SelectionAdd{{Ref: selRef("transactions", "t2")}}, nil)
			return err
		}},
		{"UpdateAnalysis", func(uc *Usecase, f *fakes, id uuid.UUID, v int) error {
			_, err := uc.UpdateAnalysis(f.Ctx, id, v, "autre analyse", "")
			return err
		}},
		{"Export", func(uc *Usecase, f *fakes, id uuid.UUID, v int) error {
			_, err := uc.Export(f.Ctx, id, v)
			return err
		}},
	}

	for _, status := range []models.SarJobStatus{models.SarJobStatusPending, models.SarJobStatusRunning} {
		for _, w := range writers {
			status, w := status, w
			t.Run(string(status)+"/"+w.name, func(t *testing.T) {
				uc, f, declId := genReady(t, true)
				f.Decisions.Add(f.OrgId, "transactions", map[string]any{"object_id": "t2", "amount": 7}, time.Now())
				f.SeedJob(t, declId, models.SarJobKindGenerate, status)
				version := f.Repo.Declarations[declId].Version

				err := w.call(uc, f, declId, version)
				require.ErrorIs(t, err, ErrGenerationInProgress)
				assert.ErrorIs(t, err, models.ConflictError)
				assert.Equal(t, version, f.Repo.Declarations[declId].Version)
			})
		}
	}
}

func TestResumeJob_RefusedWhenAlreadyGenerated(t *testing.T) {
	uc, f, declId := genReady(t, false)
	failed := f.SeedJob(t, declId, models.SarJobKindGenerate, models.SarJobStatusFailed)
	f.Clock.Advance(time.Minute)
	f.SeedDoneGeneration(t, declId)

	_, err := uc.ResumeJob(f.Ctx, failed.Id)
	require.ErrorIs(t, err, ErrAlreadyGenerated)
	assert.Equal(t, models.SarJobStatusFailed, f.Gen.Jobs[failed.Id].Status)
	assert.Empty(t, f.Enqueuer.Calls)
}

func TestGetJob_StaleRunningReportedFailed(t *testing.T) {
	uc, f, declId := genReady(t, false)
	job := f.SeedJob(t, declId, models.SarJobKindGenerate, models.SarJobStatusRunning)
	f.Clock.Advance(61 * time.Minute)

	st, err := uc.GetJob(f.Ctx, job.Id)
	require.NoError(t, err)
	assert.Equal(t, models.SarJobStatusFailed, st.Job.Status)
	assert.NotEmpty(t, st.LastError)

	// A writer does not stay blocked by the stale job: it fails it and goes on.
	_, err = uc.UpdateFields(f.Ctx, declId, f.Repo.Declarations[declId].Version,
		[]core.Fill{{Path: "organisme.libelle", Value: "X"}})
	require.NoError(t, err)
	stored := f.Gen.Jobs[job.Id]
	assert.Equal(t, models.SarJobStatusFailed, stored.Status)
	assert.Contains(t, stored.Error, "ne répond plus")
}

func TestRequiredSettingPaths_RealSpec(t *testing.T) {
	uc, _ := newTestUsecase(t)
	key, err := core.ParseFormatKey(tracfinFormatKey)
	require.NoError(t, err)
	format, ok := uc.formatGetter(key)
	require.True(t, ok)

	base := requiredSettingPaths(format, nil)
	assert.Contains(t, base, core.Path("declarant.coordonnees.tel"), "format-provided list")
	assert.Contains(t, base, core.Path("organisme.libelle"))
	assert.NotContains(t, base, core.Path("contact.coordonnees.tel"), "optional group absent")

	withContact := requiredSettingPaths(format, map[string]string{"contact.coordonnees.nom": "X"})
	assert.Contains(t, withContact, core.Path("contact.coordonnees.tel"), "optional group present")
}

func TestStartGeneration_InvalidRequiredSettingListed(t *testing.T) {
	uc, f, declId := genReady(t, false)
	bad := genCompleteSettings()
	bad["organisme.adresse.pays"] = "pas-un-code"
	genSetSettings(f, bad)

	_, err := uc.StartGeneration(f.Ctx, declId)
	require.ErrorIs(t, err, models.BadParameterError)
	require.ErrorContains(t, err, "organisme.adresse.pays")
	assert.Zero(t, f.Gen.JobCreates)
}
