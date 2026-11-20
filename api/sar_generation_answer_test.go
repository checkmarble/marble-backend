package api

import (
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/usecases/sargen"
	"github.com/checkmarble/marble-backend/usecases/sargen/core"
)

func TestHandleAnswerQuestion_Success(t *testing.T) {
	qid := uuid.New()
	f := &fakeSarGenerationUsecase{State: sarHTTPState(uuid.New(), uuid.New())}
	w := doSarRequest(newSarTestRouter(t, f), http.MethodPost, "/sar-questions/"+qid.String()+"/answer",
		map[string]any{"version": 3, "value": "Banque Test", "path": "context.general.serviceEmetteur"})

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	calls := f.Called("AnswerQuestion")
	require.Len(t, calls, 1)
	assert.Equal(t, qid, calls[0].Args[0])
	assert.Equal(t, 3, calls[0].Args[1])
	assert.Equal(t, "Banque Test", calls[0].Args[2])
	assert.Equal(t, core.Path("context.general.serviceEmetteur"), calls[0].Args[3])
}

func TestHandleAnswerQuestion_PathOptional(t *testing.T) {
	f := &fakeSarGenerationUsecase{State: sarHTTPState(uuid.New(), uuid.New())}
	w := doSarRequest(newSarTestRouter(t, f), http.MethodPost, "/sar-questions/"+uuid.NewString()+"/answer",
		map[string]any{"version": 1, "value": "x"})
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, core.Path(""), f.Called("AnswerQuestion")[0].Args[3])
}

func TestHandleAnswerQuestion_BadInput(t *testing.T) {
	qid := uuid.NewString()
	for name, body := range map[string]map[string]any{
		"no version": {"value": "x"},
		"no value":   {"version": 1},
		"empty":      {"version": 1, "value": ""},
	} {
		t.Run(name, func(t *testing.T) {
			f := &fakeSarGenerationUsecase{}
			w := doSarRequest(newSarTestRouter(t, f), http.MethodPost, "/sar-questions/"+qid+"/answer", body)
			assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
			assert.Empty(t, f.Calls)
		})
	}
	f := &fakeSarGenerationUsecase{}
	w := doSarRequest(newSarTestRouter(t, f), http.MethodPost, "/sar-questions/not-a-uuid/answer",
		map[string]any{"version": 1, "value": "x"})
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Empty(t, f.Calls)
}

func TestHandleAnswerQuestion_ErrorCodes(t *testing.T) {
	for _, tc := range []struct {
		name   string
		err    error
		status int
		code   string
	}{
		{"invalid request", models.NewSarError(models.BadParameterError, models.SarCodeInvalidRequest,
			"cette question se tranche par une de ses options"), http.StatusBadRequest, "sar_invalid_request"},
		{"invalid field", sargen.NewIssuesError(models.SarCodeInvalidField, []core.Issue{{
			Path: "context.information.activiteLPS", Code: "bad_type", Message: "booléen attendu",
		}}), http.StatusBadRequest, "sar_invalid_field"},
		{"stale version", models.ErrSarVersionConflict, http.StatusConflict, "sar_version_conflict"},
		{"closed", models.NewSarError(models.ConflictError, models.SarCodeQuestionClosed, "cette question est déjà fermée"),
			http.StatusConflict, "sar_question_closed"},
		{"other organization", models.NewSarError(models.ForbiddenError, models.SarCodeForbidden, "interdit"),
			http.StatusForbidden, "sar_forbidden"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := &fakeSarGenerationUsecase{}
			f.Fail("AnswerQuestion", tc.err)
			w := doSarRequest(newSarTestRouter(t, f), http.MethodPost, "/sar-questions/"+uuid.NewString()+"/answer",
				map[string]any{"version": 1, "value": "x"})
			require.Equal(t, tc.status, w.Code, w.Body.String())
			body := sarHTTPDecode(t, w)
			assert.Equal(t, tc.code, body["error_code"])
			if tc.code == "sar_invalid_field" {
				details, _ := body["details"].(map[string]any)
				assert.NotEmpty(t, details["issues"])
			}
		})
	}
}
