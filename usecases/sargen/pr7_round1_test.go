package sargen

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/usecases/sargen/core"
)

// Round 1: the readiness of the settings is the rule of StartGeneration.
func TestSettingsReadiness_CompleteMatchesStartGeneration(t *testing.T) {
	complete := genCompleteSettings()
	with := func(mut func(m map[string]string)) map[string]string {
		m := genCompleteSettings()
		mut(m)
		return m
	}
	cases := []struct {
		name         string
		values       map[string]string
		complete     bool
		wantMissing  string
		wantInvalid  string
		wantGenerate bool
	}{
		{name: "empty", values: map[string]string{}, complete: false, wantMissing: "organisme.libelle"},
		{name: "partial", values: map[string]string{"declarant.coordonnees.nom": "Dupont"}, complete: false, wantMissing: "organisme.libelle"},
		{name: "invalid required", values: with(func(m map[string]string) { m["organisme.adresse.pays"] = "X" }),
			complete: false, wantInvalid: "organisme.adresse.pays"},
		{name: "blocking non-required stored value", values: with(func(m map[string]string) { m["unknown.path"] = "x" }),
			complete: true},
		{name: "complete", values: complete, complete: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			uc, f := newTestUsecase(t)
			genSetSettings(f, tc.values)
			declId := createFresh(t, uc, f).Declaration.Id
			genAnalyse(t, uc, f, declId)
			genSelect(t, uc, f, declId, "t1")

			ready, err := uc.SettingsReadiness(f.Ctx, tracfinFamily, tc.values)
			require.NoError(t, err)
			assert.Equal(t, tc.complete, ready.Complete)
			if tc.wantMissing != "" {
				assert.Contains(t, ready.Missing, core.Path(tc.wantMissing))
			}
			if tc.wantInvalid != "" {
				assert.Contains(t, ready.Invalid, core.Path(tc.wantInvalid))
			}

			_, err = uc.StartGeneration(f.Ctx, declId)
			refusedForSettings := false
			var ie *IssuesError
			if errors.As(err, &ie) {
				for _, i := range ie.SarIssues() {
					refusedForSettings = refusedForSettings ||
						i.Code == CodePreconditionSettingMissing || i.Code == CodePreconditionSettingInvalid
				}
			}
			assert.Equal(t, tc.complete, !refusedForSettings, "complete <=> StartGeneration not refused for settings (err: %v)", err)
		})
	}
}

// A stale active job reads failed and is not the active job.
func TestDeclarationState_StaleJobIsNotActive(t *testing.T) {
	uc, f, declId := genReady(t, false)
	job := f.SeedJob(t, declId, models.SarJobKindGenerate, models.SarJobStatusRunning)
	f.Clock.Advance(uc.staleAfter() + time.Minute)

	got, err := uc.GetDeclaration(f.Ctx, declId)
	require.NoError(t, err)
	require.NotNil(t, got.Generation)
	assert.Equal(t, models.SarJobStatusFailed, got.Generation.Status)
	assert.Equal(t, job.Id, got.Generation.JobId)
	assert.Nil(t, got.ActiveJobId, "a stale job is not active")
}

// A review job newer than a done generate job does not hide the generation.
func TestDeclarationState_ReviewNewerThanDoneGenerate(t *testing.T) {
	uc, f, declId := genReady(t, false)
	gen := f.SeedJob(t, declId, models.SarJobKindGenerate, models.SarJobStatusDone)
	f.SeedJob(t, declId, models.SarJobKindReview, models.SarJobStatusRunning)

	got, err := uc.GetDeclaration(f.Ctx, declId)
	require.NoError(t, err)
	require.NotNil(t, got.Generation)
	assert.Equal(t, gen.Id, got.Generation.JobId)
	assert.Equal(t, models.SarJobStatusDone, got.Generation.Status)
}
