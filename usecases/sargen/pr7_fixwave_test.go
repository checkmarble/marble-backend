package sargen

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/cockroachdb/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/usecases/sargen/core"
)

// Fix wave PR 7 (verification-jobs item 6, verification-api items 1 and 2).
// The tests reach the state and the refusals through JSON or through an
// interface, so they compile before the fix adds the fields.

// pr7StateJSON returns the top-level JSON object of a declaration state.
func pr7StateJSON(t *testing.T, st DeclarationState) map[string]any {
	t.Helper()
	raw, err := json.Marshal(st)
	require.NoError(t, err)
	var out map[string]any
	require.NoError(t, json.Unmarshal(raw, &out))
	return out
}

// pr7Key returns the value of the top-level key name (case-insensitive) and
// whether the key is present at all.
func pr7Key(m map[string]any, name string) (any, bool) {
	for k, v := range m {
		if strings.EqualFold(k, name) {
			return v, true
		}
	}
	return nil, false
}

// pr7IssuesCarrier is what a refused write must expose: its structured issues.
type pr7IssuesCarrier interface {
	SarIssues() []core.Issue
}

// ---- P1: generation state on the declaration state ----

// PR 7 I1 (jobs item 6): a declaration never generated exposes generation: null.
func TestDeclarationState_GenerationNullWhenNeverGenerated(t *testing.T) {
	uc, f := newTestUsecase(t)
	st := createFresh(t, uc, f)

	got, err := uc.GetDeclaration(f.Ctx, st.Declaration.Id)
	require.NoError(t, err)
	m := pr7StateJSON(t, got)

	gen, ok := pr7Key(m, "generation")
	require.True(t, ok, "the state must expose a generation key")
	assert.Nil(t, gen, "no generate job yet: generation must be null")
}

// The generation of a declaration is the last generate job: status, job id and
// the public error of the job.
func TestDeclarationState_GenerationReportsLastGenerateJob(t *testing.T) {
	cases := []struct {
		name   string
		status models.SarJobStatus
		errMsg string
	}{
		{name: "pending", status: models.SarJobStatusPending},
		{name: "running", status: models.SarJobStatusRunning},
		{name: "failed with public error", status: models.SarJobStatusFailed, errMsg: "fournisseur indisponible"},
		{name: "done", status: models.SarJobStatusDone},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			uc, f, declId := genReady(t, false)
			job := f.SeedJob(t, declId, models.SarJobKindGenerate, models.SarJobStatusPending)
			genMarkJob(t, f, job.Id, tc.status, tc.errMsg)

			got, err := uc.GetDeclaration(f.Ctx, declId)
			require.NoError(t, err)
			m := pr7StateJSON(t, got)

			raw, ok := pr7Key(m, "generation")
			require.True(t, ok, "the state must expose a generation key")
			gen, ok := raw.(map[string]any)
			require.True(t, ok, "generation must be an object, got %T", raw)
			assert.Equal(t, string(tc.status), gen["status"])
			assert.Equal(t, job.Id.String(), gen["job_id"])
			if tc.errMsg != "" {
				assert.Equal(t, tc.errMsg, gen["error"], "the public error of the job is exposed")
			}
		})
	}
}

// A review job is not a generation: it does not feed generation.
func TestDeclarationState_GenerationIgnoresReviewJobs(t *testing.T) {
	uc, f, declId := genReady(t, false)
	f.SeedJob(t, declId, models.SarJobKindReview, models.SarJobStatusRunning)

	got, err := uc.GetDeclaration(f.Ctx, declId)
	require.NoError(t, err)
	gen, ok := pr7Key(pr7StateJSON(t, got), "generation")
	require.True(t, ok, "the state must expose a generation key")
	assert.Nil(t, gen, "a review job alone means never generated")
}

// ---- P3: refused field writes carry their structured issues ----

// PR 7 I2 (api item 1): a field write refused by validation exposes its issues
// (path, code, message) instead of one concatenated string.
func TestUpdateFields_RefusalExposesStructuredIssues(t *testing.T) {
	uc, f := newTestUsecase(t)
	st := createFresh(t, uc, f)

	_, err := uc.UpdateFields(f.Ctx, st.Declaration.Id, st.Declaration.Version, []core.Fill{
		{Path: "declarant.coordonnees.nom", Value: strings.Repeat("x", 5000)},
	})
	require.ErrorIs(t, err, models.BadParameterError)
	assertCleanRefusal(t, err)

	var carrier pr7IssuesCarrier
	require.True(t, errors.As(err, &carrier), "the refusal must expose its issues, got %v", err)
	issues := carrier.SarIssues()
	require.Len(t, issues, 1)
	assert.Equal(t, core.Path("declarant.coordonnees.nom"), issues[0].Path)
	assert.NotEmpty(t, issues[0].Code)
	assert.NotEmpty(t, issues[0].Message)
}

// The analysis write is refused the same way (max_length of the analysis).
func TestUpdateAnalysis_RefusalExposesStructuredIssues(t *testing.T) {
	uc, f := newTestUsecase(t)
	st := createFresh(t, uc, f)

	_, err := uc.UpdateAnalysis(f.Ctx, st.Declaration.Id, st.Declaration.Version,
		strings.Repeat("a", 23001), "")
	require.ErrorIs(t, err, models.BadParameterError)
	assertCleanRefusal(t, err)

	var carrier pr7IssuesCarrier
	require.True(t, errors.As(err, &carrier), "the refusal must expose its issues, got %v", err)
	issues := carrier.SarIssues()
	require.NotEmpty(t, issues)
	assert.Equal(t, core.Path("analyse"), issues[0].Path)
}

// assertCleanRefusal: the message has no transaction wrapper and no sentinel
// suffix, it is what the analyst reads.
func assertCleanRefusal(t *testing.T, err error) {
	t.Helper()
	msg := err.Error()
	assert.NotContains(t, msg, "Error executing transaction")
	assert.False(t, strings.HasSuffix(msg, models.BadParameterError.Error()),
		"the message must not end with the sentinel text: %q", msg)
	assert.NotEmpty(t, strings.TrimSpace(msg))
}
