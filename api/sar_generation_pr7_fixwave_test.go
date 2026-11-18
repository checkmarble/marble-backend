package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/usecases/sargen/core"
)

// Fix wave PR 7 (verification-jobs item 6, verification-api items 1-3).

// pr7Strings returns the JSON array under key as strings; the key must exist.
func pr7Strings(t *testing.T, body map[string]any, key string) []string {
	t.Helper()
	raw, ok := body[key]
	require.True(t, ok, "response must expose %q", key)
	list, ok := raw.([]any)
	require.True(t, ok, "%q must be an array, got %T", key, raw)
	out := make([]string, 0, len(list))
	for _, v := range list {
		s, ok := v.(string)
		require.True(t, ok, "%q items must be strings, got %T", key, v)
		out = append(out, s)
	}
	return out
}

// pr7CompleteSettingsValues fills every required TRACFIN setting.
func pr7CompleteSettingsValues() map[string]string {
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

// ---- P1: generation on the declaration state ----

// PR 7 I1 (jobs item 6): a never generated declaration exposes generation: null.
func TestHandleGetSarDeclaration_GenerationNullWhenNeverGenerated(t *testing.T) {
	id := uuid.New()
	f := &fakeSarGenerationUsecase{State: sarHTTPState(id, uuid.New())}
	r := newSarTestRouter(t, f)

	w := doSarRequest(r, http.MethodGet, "/sar-declarations/"+id.String(), nil)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	body := sarHTTPDecode(t, w)
	require.Contains(t, body, "generation", "the state must expose generation")
	assert.Nil(t, body["generation"])
}

// ---- P2: completeness of the organization settings ----

// PR 7 I4 (api item 3): GET exposes the missing required paths and complete.
func TestHandleGetSarOrgSettings_ExposesMissingAndComplete(t *testing.T) {
	t.Run("empty settings are incomplete", func(t *testing.T) {
		f := &fakeSarGenerationUsecase{}
		r := newSarTestRouter(t, f)

		w := doSarRequest(r, http.MethodGet, "/sar-format-settings/tracfin", nil)

		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		body := sarHTTPDecode(t, w)
		assert.Equal(t, false, body["complete"])
		missing := pr7Strings(t, body, "missing")
		assert.Contains(t, missing, "organisme.libelle")
		assert.Contains(t, missing, "declarant.coordonnees.tel")
	})

	t.Run("all required settings set is complete", func(t *testing.T) {
		f := &fakeSarGenerationUsecase{Settings: models.SarOrgFormatSettings{
			FormatFamily: "tracfin", Values: pr7CompleteSettingsValues(),
		}}
		r := newSarTestRouter(t, f)

		w := doSarRequest(r, http.MethodGet, "/sar-format-settings/tracfin", nil)

		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		body := sarHTTPDecode(t, w)
		assert.Equal(t, true, body["complete"])
		assert.Empty(t, pr7Strings(t, body, "missing"))
	})
}

// PUT returns the same completeness of the values it stores.
func TestHandlePutSarOrgSettings_ExposesMissingAndComplete(t *testing.T) {
	t.Run("partial values are incomplete", func(t *testing.T) {
		partial := map[string]string{"declarant.coordonnees.nom": "Dupont"}
		f := &fakeSarGenerationUsecase{Settings: models.SarOrgFormatSettings{
			FormatFamily: "tracfin", Values: partial,
		}}
		r := newSarTestRouter(t, f)

		w := doSarRequest(r, http.MethodPut, "/sar-format-settings/tracfin", map[string]any{"values": partial})

		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		body := sarHTTPDecode(t, w)
		assert.Equal(t, false, body["complete"])
		assert.Contains(t, pr7Strings(t, body, "missing"), "organisme.libelle")
	})

	t.Run("complete values are complete", func(t *testing.T) {
		full := pr7CompleteSettingsValues()
		f := &fakeSarGenerationUsecase{Settings: models.SarOrgFormatSettings{
			FormatFamily: "tracfin", Values: full,
		}}
		r := newSarTestRouter(t, f)

		w := doSarRequest(r, http.MethodPut, "/sar-format-settings/tracfin", map[string]any{"values": full})

		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		body := sarHTTPDecode(t, w)
		assert.Equal(t, true, body["complete"])
		assert.Empty(t, pr7Strings(t, body, "missing"))
	})
}

// ---- P3: refused field writes carry structured issues ----

// pr7IssuesErr stands for a refused write as the usecase returns it: a
// BadParameterError exposing its issues.
type pr7IssuesErr struct{ issues []core.Issue }

func (e pr7IssuesErr) Error() string           { return "le champ est trop long" }
func (e pr7IssuesErr) Unwrap() error           { return models.BadParameterError }
func (e pr7IssuesErr) SarIssues() []core.Issue { return e.issues }

// PR 7 I2 (api item 1): a refused write is a 400 with details.issues, the same
// shape as PUT /sar-format-settings, and a clean French message.
func TestHandleRefusedWrites_ReturnStructuredIssues(t *testing.T) {
	issue := core.Issue{
		Path: "declarant.coordonnees.nom", Code: "max_length", Severity: core.SeverityBlocking,
		Message: "50 caractères au maximum (5000 saisis)",
	}

	t.Run("field write", func(t *testing.T) {
		id := uuid.New()
		f := &fakeSarGenerationUsecase{}
		f.Fail("UpdateFields", pr7IssuesErr{issues: []core.Issue{issue}})
		r := newSarTestRouter(t, f)

		w := doSarRequest(r, http.MethodPatch, "/sar-declarations/"+id.String()+"/fields",
			map[string]any{"version": 3, "fields": []map[string]string{{"path": "declarant.coordonnees.nom", "value": "x"}}})

		assertPr7StructuredRefusal(t, w, issue)
	})

	t.Run("analysis write", func(t *testing.T) {
		id := uuid.New()
		f := &fakeSarGenerationUsecase{}
		f.Fail("UpdateAnalysis", pr7IssuesErr{issues: []core.Issue{issue}})
		r := newSarTestRouter(t, f)

		w := doSarRequest(r, http.MethodPut, "/sar-declarations/"+id.String()+"/analysis",
			map[string]any{"version": 3, "analysis": "x"})

		assertPr7StructuredRefusal(t, w, issue)
	})
}

// assertPr7StructuredRefusal checks a 400 carrying details.issues and a clean
// message (no transaction wrapper).
func assertPr7StructuredRefusal(t *testing.T, w *httptest.ResponseRecorder, issue core.Issue) {
	t.Helper()
	require.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
	body := sarHTTPDecode(t, w)
	msg, _ := body["message"].(string)
	assert.NotEmpty(t, msg)
	assert.NotContains(t, msg, "Error executing transaction")

	details, ok := body["details"].(map[string]any)
	require.True(t, ok, "a refused write must carry details, body: %s", w.Body.String())
	raw, ok := details["issues"].([]any)
	require.True(t, ok, "details.issues must be an array, body: %s", w.Body.String())
	require.Len(t, raw, 1)
	got, ok := raw[0].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, string(issue.Path), got["path"])
	assert.Equal(t, issue.Code, got["code"])
	assert.Equal(t, issue.Message, got["message"])
}

// ---- P4: bounded extra_info ----

// PR 7 I3 (api item 2): extra_info is bounded at 10000 characters: above is a
// 400 that never reaches the usecase, at the limit it is accepted.
func TestHandleUpdateSarAnalysis_ExtraInfoBounded(t *testing.T) {
	id := uuid.New()
	caseId := uuid.New()

	t.Run("above the limit refused", func(t *testing.T) {
		f := &fakeSarGenerationUsecase{State: sarHTTPState(id, caseId)}
		r := newSarTestRouter(t, f)

		w := doSarRequest(r, http.MethodPut, "/sar-declarations/"+id.String()+"/analysis",
			map[string]any{"version": 3, "analysis": "analyse", "extra_info": strings.Repeat("x", 10001)})

		assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
		assert.Empty(t, f.Called("UpdateAnalysis"), "the usecase must not be reached")
	})

	t.Run("at the limit accepted", func(t *testing.T) {
		f := &fakeSarGenerationUsecase{State: sarHTTPState(id, caseId)}
		r := newSarTestRouter(t, f)

		w := doSarRequest(r, http.MethodPut, "/sar-declarations/"+id.String()+"/analysis",
			map[string]any{"version": 3, "analysis": "analyse", "extra_info": strings.Repeat("x", 10000)})

		assert.Equal(t, http.StatusOK, w.Code, w.Body.String())
		assert.Len(t, f.Called("UpdateAnalysis"), 1)
	})
}
