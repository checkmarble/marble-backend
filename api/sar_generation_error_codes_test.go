package api

import (
	"net/http"
	"testing"

	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/usecases/sargen"
	"github.com/checkmarble/marble-backend/usecases/sargen/core"
)

// Every refusal of the SAR endpoints carries a stable error_code and a clean
// French message, without the wrapper text of the layers it crossed.

func TestSarErrorCodes_PutSettingsRefusal(t *testing.T) {
	issues := []core.Issue{{
		Path: "organisme.numIdPro", Code: "pattern", Message: "format invalide", Severity: core.SeverityBlocking,
	}}
	f := &fakeSarGenerationUsecase{Issues: issues}
	f.Fail("PutOrgSettings", sargen.NewIssuesErrorWithMessage(models.SarCodeInvalidSettings, "réglages invalides", issues))

	w := doSarRequest(newSarTestRouter(t, f), http.MethodPut, "/sar-format-settings/tracfin",
		map[string]any{"values": map[string]string{"organisme.numIdPro": "x"}})

	require.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
	body := sarHTTPDecode(t, w)
	assert.Equal(t, "sar_invalid_settings", body["error_code"])
	assert.NotContains(t, body["message"], "bad parameter")
	assert.Equal(t, "réglages invalides : organisme.numIdPro : format invalide", body["message"])
	assert.Len(t, body["details"].(map[string]any)["issues"], 1)
}

func TestSarErrorCodes_ExportRefusal(t *testing.T) {
	f := &fakeSarGenerationUsecase{}
	f.Fail("Export", errors.Wrap(sargen.NewIssuesErrorWithMessage(models.SarCodeExportBlocked,
		"l'export est impossible : des erreurs bloquantes subsistent",
		[]core.Issue{{Path: "a.b", Code: "c", Severity: core.SeverityBlocking, Message: "m"}}),
		"Error executing transaction"))

	w := doSarRequest(newSarTestRouter(t, f), http.MethodPost,
		"/sar-declarations/"+uuid.NewString()+"/exports", map[string]any{"version": 3})

	require.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
	body := sarHTTPDecode(t, w)
	assert.Equal(t, "sar_export_blocked", body["error_code"])
	assert.NotContains(t, body["message"], "Error executing transaction")
}

// The version conflict is raised inside a transaction: the executor wraps it,
// and a unique-constraint failure may append ": duplicate value".
func TestSarErrorCodes_VersionConflictInsideATransaction(t *testing.T) {
	f := &fakeSarGenerationUsecase{}
	f.Fail("UpdateFields", errors.Wrap(
		errors.Wrap(models.ErrSarVersionConflict, "duplicate value"), "Error executing transaction"))

	w := doSarRequest(newSarTestRouter(t, f), http.MethodPatch,
		"/sar-declarations/"+uuid.NewString()+"/fields",
		map[string]any{"version": 3, "fields": []map[string]string{{"path": "declarant.coordonnees.nom", "value": "x"}}})

	require.Equal(t, http.StatusConflict, w.Code, w.Body.String())
	body := sarHTTPDecode(t, w)
	assert.Equal(t, "la déclaration a été modifiée entre-temps", body["message"])
	assert.Equal(t, "sar_version_conflict", body["error_code"])
	assert.NotContains(t, body, "details")
}

func TestSarErrorCodes_OtherRefusals(t *testing.T) {
	id := uuid.NewString()
	for _, tc := range []struct {
		name, method, path, usecase string
		body                        any
		err                         error
		status                      int
		code                        string
	}{
		{"generation in progress", http.MethodPost, "/sar-declarations/" + id + "/generate", "StartGeneration", nil,
			sargen.ErrGenerationInProgress, http.StatusConflict, "sar_generation_in_progress"},
		{"job already active", http.MethodPost, "/sar-declarations/" + id + "/generate", "StartGeneration", nil,
			errors.Wrap(models.ErrSarJobAlreadyActive, "Error executing transaction"), http.StatusConflict, "sar_generation_in_progress"},
		{"already generated", http.MethodPost, "/sar-declarations/" + id + "/generate", "StartGeneration", nil,
			sargen.ErrAlreadyGenerated, http.StatusConflict, "sar_already_generated"},
		{"selection frozen", http.MethodPatch, "/sar-declarations/" + id + "/selection", "UpdateSelection",
			map[string]any{"version": 1}, sargen.ErrSelectionFrozen, http.StatusConflict, "sar_selection_frozen"},
		{"not found", http.MethodPost, "/sar-declarations/" + id + "/items", "AddItem",
			map[string]any{"version": 1, "container": "a.b"},
			models.NewSarError(models.NotFoundError, models.SarCodeNotFound, "l'élément n'existe pas"),
			http.StatusNotFound, "sar_not_found"},
		{"forbidden", http.MethodGet, "/sar-declarations/" + id, "GetDeclaration", nil,
			models.NewSarError(models.ForbiddenError, models.SarCodeForbidden, "cette déclaration n'appartient pas à votre organisation"),
			http.StatusForbidden, "sar_forbidden"},
		{"unknown format", http.MethodPost, "/cases/" + id + "/sar-declarations", "CreateDeclaration",
			map[string]any{"format_key": "x", "report_type": "y"},
			models.NewSarError(models.BadParameterError, models.SarCodeUnknownFormat, "format inconnu : x"),
			http.StatusBadRequest, "sar_unknown_format"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := &fakeSarGenerationUsecase{}
			f.Fail(tc.usecase, tc.err)
			w := doSarRequest(newSarTestRouter(t, f), tc.method, tc.path, tc.body)

			require.Equal(t, tc.status, w.Code, w.Body.String())
			body := sarHTTPDecode(t, w)
			assert.Equal(t, tc.code, body["error_code"])
			assert.NotContains(t, body["message"], "Error executing transaction")
			assert.NotContains(t, body["message"], "conflict")
		})
	}
}

// Generation preconditions are refused at once, as issues of three kinds.
func TestSarErrorCodes_GenerationPreconditions(t *testing.T) {
	issues := []core.Issue{
		{Path: sargen.PreconditionAnalysisPath, Code: sargen.CodePreconditionAnalysisEmpty, Severity: core.SeverityBlocking,
			Message: "l'analyse est vide : renseignez-la avant de générer"},
		{Path: sargen.PreconditionSelectionPath, Code: sargen.CodePreconditionSelectionEmpty, Severity: core.SeverityBlocking,
			Message: "la sélection est vide : sélectionnez des objets avant de générer"},
		{Path: "organisme.libelle", Code: sargen.CodePreconditionSettingMissing, Severity: core.SeverityBlocking,
			Message: "réglage de l'organisation manquant"},
	}
	for _, route := range []struct{ name, method, path, usecase string }{
		{"generate", http.MethodPost, "/sar-declarations/" + uuid.NewString() + "/generate", "StartGeneration"},
		{"resume", http.MethodPost, "/sar-generation-jobs/" + uuid.NewString() + "/resume", "ResumeJob"},
	} {
		t.Run(route.name, func(t *testing.T) {
			f := &fakeSarGenerationUsecase{}
			f.Fail(route.usecase, errors.Wrap(
				sargen.NewIssuesErrorWithMessage(models.SarCodeGenerationPrecondition, "la génération est impossible", issues),
				"Error executing transaction"))
			w := doSarRequest(newSarTestRouter(t, f), route.method, route.path, nil)

			require.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
			body := sarHTTPDecode(t, w)
			assert.Equal(t, "sar_generation_preconditions", body["error_code"])
			got := body["details"].(map[string]any)["issues"].([]any)
			require.Len(t, got, 3)
			var paths []any
			for _, i := range got {
				paths = append(paths, i.(map[string]any)["path"])
			}
			assert.Equal(t, []any{"analysis", "selection", "organisme.libelle"}, paths)
		})
	}
}

// Bare sentinels coming from the layers below the usecase (repository, case
// security) get a code and a generic French message, never the Go chain.
func TestSarErrorCodes_BareSentinelsFromLowerLayers(t *testing.T) {
	id := uuid.NewString()
	for _, tc := range []struct {
		name   string
		err    error
		status int
		code   string
		msg    string
	}{
		{"repository not found",
			errors.Wrap(models.NotFoundError, "found no object of type models.SarDeclaration"),
			http.StatusNotFound, "sar_not_found", "élément introuvable"},
		{"case access forbidden",
			errors.Wrap(models.ForbiddenError, "User does not have access to case's inbox"),
			http.StatusForbidden, "sar_forbidden", "accès refusé"},
		{"bare conflict inside a transaction",
			errors.Wrap(models.ConflictError, "Error executing transaction"),
			http.StatusConflict, "sar_conflict", "conflit avec l'état actuel de la déclaration"},
		{"bare bad parameter",
			errors.Wrap(models.BadParameterError, "some go detail"),
			http.StatusBadRequest, "sar_invalid_request", "requête invalide"},
		{"unprocessable",
			errors.Wrap(models.UnprocessableEntityError, "some go detail"),
			http.StatusUnprocessableEntity, "sar_unprocessable", "requête impossible à traiter"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := &fakeSarGenerationUsecase{}
			f.Fail("GetDeclaration", tc.err)
			w := doSarRequest(newSarTestRouter(t, f), http.MethodGet, "/sar-declarations/"+id, nil)

			require.Equal(t, tc.status, w.Code, w.Body.String())
			body := sarHTTPDecode(t, w)
			assert.Equal(t, tc.code, body["error_code"])
			assert.Equal(t, tc.msg, body["message"])
		})
	}
}

// A SarError of a kind mapped to no status is a 500 with a code and no detail.
func TestSarErrorCodes_UnmappedKindIsInternal(t *testing.T) {
	f := &fakeSarGenerationUsecase{}
	f.Fail("GetDeclaration", models.NewSarError(errors.New("secret go detail"), models.SarCodeInvalidRequest, "msg"))
	w := doSarRequest(newSarTestRouter(t, f), http.MethodGet, "/sar-declarations/"+uuid.NewString(), nil)

	require.Equal(t, http.StatusInternalServerError, w.Code, w.Body.String())
	body := sarHTTPDecode(t, w)
	assert.Equal(t, "sar_internal", body["error_code"])
	assert.NotContains(t, w.Body.String(), "secret")
}
