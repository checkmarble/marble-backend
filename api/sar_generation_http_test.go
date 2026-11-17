package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/usecases"
	"github.com/checkmarble/marble-backend/usecases/sargen"
	"github.com/checkmarble/marble-backend/usecases/sargen/core"
	"github.com/checkmarble/marble-backend/utils"
)

const sarHTTPFormatKey = "tracfin:FR:1.4.0"

// sarHTTPState is a declaration state at version 3 with an empty document.
func sarHTTPState(id, caseId uuid.UUID) sargen.DeclarationState {
	return sargen.DeclarationState{
		Declaration: models.SarDeclaration{
			Id: id, CaseId: caseId, FormatKey: sarHTTPFormatKey, ReportType: "declaration", Version: 3,
		},
		Doc: core.NewDocument(),
	}
}

// sarHTTPDecode decodes the JSON body of a response into a map.
func sarHTTPDecode(t *testing.T, w *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var out map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &out), "body: %s", w.Body.String())
	return out
}

// sarHTTPFindKeys returns the paths of every object key (at any depth) that is
// in banned.
func sarHTTPFindKeys(v any, banned map[string]bool, path string) []string {
	var found []string
	switch x := v.(type) {
	case map[string]any:
		for k, val := range x {
			if banned[strings.ToLower(k)] {
				found = append(found, path+"."+k)
			}
			found = append(found, sarHTTPFindKeys(val, banned, path+"."+k)...)
		}
	case []any:
		for i, val := range x {
			found = append(found, sarHTTPFindKeys(val, banned, fmt.Sprintf("%s[%d]", path, i))...)
		}
	}
	return found
}

// sarTestBlobCloser is a models.Blob reader that records its Close.
type sarTestBlobCloser struct {
	*bytes.Reader
	closed *bool
}

func (c sarTestBlobCloser) Close() error {
	*c.closed = true
	return nil
}

func TestHandleCreateDeclaration_OK(t *testing.T) {
	caseId := uuid.New()
	id := uuid.New()
	f := &fakeSarGenerationUsecase{State: sarHTTPState(id, caseId)}
	r := newSarTestRouter(t, f)

	w := doSarRequest(r, http.MethodPost, "/cases/"+caseId.String()+"/sar-declarations",
		map[string]any{"format_key": sarHTTPFormatKey, "report_type": "declaration"})

	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	body := sarHTTPDecode(t, w)
	assert.Equal(t, id.String(), body["id"])
	assert.Equal(t, caseId.String(), body["case_id"])
	assert.Equal(t, sarHTTPFormatKey, body["format_key"])
	assert.EqualValues(t, 3, body["version"])
	assert.Contains(t, body, "document")
	assert.Contains(t, body, "todo")

	calls := f.Called("CreateDeclaration")
	require.Len(t, calls, 1)
	assert.Equal(t, caseId, calls[0].Args[0])
	assert.Equal(t, sarHTTPFormatKey, calls[0].Args[1])
	assert.Equal(t, "declaration", calls[0].Args[2])
}

func TestHandleUpdateFields_Conflict409(t *testing.T) {
	id := uuid.New()
	f := &fakeSarGenerationUsecase{}
	f.Fail("UpdateFields", models.ErrSarVersionConflict)
	r := newSarTestRouter(t, f)

	w := doSarRequest(r, http.MethodPatch, "/sar-declarations/"+id.String()+"/fields",
		map[string]any{"version": 3, "fields": []map[string]string{{"path": "a.b", "value": "x"}}})

	assert.Equal(t, http.StatusConflict, w.Code, w.Body.String())
}

func TestHandleUpdateFields_OnlyPathAndValueReachUsecase(t *testing.T) {
	id := uuid.New()
	f := &fakeSarGenerationUsecase{State: sarHTTPState(id, uuid.New())}
	r := newSarTestRouter(t, f)

	w := doSarRequest(r, http.MethodPatch, "/sar-declarations/"+id.String()+"/fields", map[string]any{
		"version": 3,
		"fields": []map[string]any{{
			"path":     "personnes.pp[pp-1].nom",
			"value":    "DUPONT",
			"origin":   "ai",
			"reviewed": true,
			"sources":  []map[string]any{{"object": map[string]string{"table": "t", "object_id": "o"}, "field": "f"}},
		}},
	})

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	calls := f.Called("UpdateFields")
	require.Len(t, calls, 1)
	assert.Equal(t, id, calls[0].Args[0])
	assert.Equal(t, 3, calls[0].Args[1])

	fills, ok := calls[0].Args[2].([]core.Fill)
	require.True(t, ok, "fills type %T", calls[0].Args[2])
	require.Len(t, fills, 1)
	assert.Equal(t, core.Path("personnes.pp[pp-1].nom"), fills[0].Path)
	assert.Equal(t, "DUPONT", fills[0].Value)
	assert.Contains(t, []core.Origin{"", core.OriginManual}, fills[0].Origin, "origin must not come from the client")
	assert.Empty(t, fills[0].Sources, "sources must not come from the client")
	assert.Empty(t, fills[0].Justification)
	assert.Empty(t, fills[0].RuleIDs)
}

func TestHandleExport_BlockingIssues400(t *testing.T) {
	id := uuid.New()
	f := &fakeSarGenerationUsecase{}
	f.Fail("Export", models.BadParameterError)
	r := newSarTestRouter(t, f)

	w := doSarRequest(r, http.MethodPost, "/sar-declarations/"+id.String()+"/exports", map[string]any{"version": 3})

	assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
}

func TestHandleGetSpec_OK(t *testing.T) {
	f := &fakeSarGenerationUsecase{}
	r := newSarTestRouter(t, f)

	w := doSarRequest(r, http.MethodGet, "/sar-formats/"+sarHTTPFormatKey+"/spec", nil)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	body := sarHTTPDecode(t, w)
	assert.Equal(t, sarHTTPFormatKey, body["format_key"])
	assert.NotNil(t, body["root"], "spec must expose the node tree")
	codeLists, ok := body["code_lists"].(map[string]any)
	require.True(t, ok, "code_lists must be an object, got %T", body["code_lists"])
	assert.NotEmpty(t, codeLists)
}

func TestHandleGetSpec_UnknownFormat404(t *testing.T) {
	f := &fakeSarGenerationUsecase{}
	r := newSarTestRouter(t, f)

	w := doSarRequest(r, http.MethodGet, "/sar-formats/tracfin:FR:9.9.9/spec", nil)

	assert.Equal(t, http.StatusNotFound, w.Code, w.Body.String())
}

func TestHandleGenerate_ReturnsJob(t *testing.T) {
	id := uuid.New()
	jobId := uuid.New()
	f := &fakeSarGenerationUsecase{Job: models.SarGenerationJob{
		Id: jobId, DeclarationId: id, Status: models.SarJobStatusPending,
	}}
	r := newSarTestRouter(t, f)

	w := doSarRequest(r, http.MethodPost, "/sar-declarations/"+id.String()+"/generate", nil)

	require.Equal(t, http.StatusAccepted, w.Code, w.Body.String())
	body := sarHTTPDecode(t, w)
	assert.Equal(t, jobId.String(), body["id"])
	assert.Equal(t, id.String(), body["declaration_id"])
	assert.Equal(t, string(models.SarJobStatusPending), body["status"])
}

func TestHandleGenerate_ActiveJobConflict409(t *testing.T) {
	id := uuid.New()
	f := &fakeSarGenerationUsecase{}
	f.Fail("StartGeneration", models.ErrSarJobAlreadyActive)
	r := newSarTestRouter(t, f)

	w := doSarRequest(r, http.MethodPost, "/sar-declarations/"+id.String()+"/generate", nil)

	assert.Equal(t, http.StatusConflict, w.Code, w.Body.String())
}

func TestHandleNavigate_Forbidden403(t *testing.T) {
	id := uuid.New()
	f := &fakeSarGenerationUsecase{}
	f.Fail("Navigate", models.ForbiddenError)
	r := newSarTestRouter(t, f)

	w := doSarRequest(r, http.MethodGet,
		"/sar-declarations/"+id.String()+"/navigate?table=transactions&object_id=tx-1&link=account", nil)

	assert.Equal(t, http.StatusForbidden, w.Code, w.Body.String())
}

func TestHandleNavigate_LimitCapped(t *testing.T) {
	id := uuid.New()
	f := &fakeSarGenerationUsecase{}
	r := newSarTestRouter(t, f)

	w := doSarRequest(r, http.MethodGet,
		"/sar-declarations/"+id.String()+"/navigate?table=transactions&object_id=tx-1&link=account&limit=500", nil)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	calls := f.Called("Navigate")
	require.Len(t, calls, 1)
	assert.Equal(t, core.ObjectRef{Table: "transactions", ObjectID: "tx-1"}, calls[0].Args[1])
	assert.Equal(t, "account", calls[0].Args[2])
	assert.Equal(t, 100, calls[0].Args[4], "limit must be capped to 100")
}

func TestHandleChooseOption_Conflict409(t *testing.T) {
	qid := uuid.New()
	f := &fakeSarGenerationUsecase{}
	f.Fail("ChooseQuestionOption", models.ErrSarVersionConflict)
	r := newSarTestRouter(t, f)

	w := doSarRequest(r, http.MethodPost, "/sar-questions/"+qid.String()+"/choose",
		map[string]any{"version": 3, "option_index": 0})

	assert.Equal(t, http.StatusConflict, w.Code, w.Body.String())
}

func TestHandleGetJob_HidesTaskInternals(t *testing.T) {
	jobId := uuid.New()
	f := &fakeSarGenerationUsecase{JobState: sargen.JobState{
		Job: models.SarGenerationJob{
			Id: jobId, DeclarationId: uuid.New(), Status: models.SarJobStatusRunning,
			Input: []byte(`{"secret_input":"do-not-leak"}`),
		},
		Tasks: []sargen.TaskSummary{{
			Id: uuid.New(), Kind: "fill", TargetPath: "a.b", Status: models.SarJobStatusDone, Attempts: 1,
		}},
		Done: 1, Total: 2, Attempt: 1, MaxAttempts: models.SarGenerationMaxAttempts,
	}}
	r := newSarTestRouter(t, f)

	w := doSarRequest(r, http.MethodGet, "/sar-generation-jobs/"+jobId.String(), nil)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	body := sarHTTPDecode(t, w)
	banned := map[string]bool{"input": true, "output": true, "trace": true}
	assert.Empty(t, sarHTTPFindKeys(body, banned, "$"))
	assert.NotContains(t, w.Body.String(), "do-not-leak")
	assert.EqualValues(t, 1, body["done"])
	assert.EqualValues(t, 2, body["total"])
}

func TestHandleInvalidUUID_400(t *testing.T) {
	f := &fakeSarGenerationUsecase{}
	r := newSarTestRouter(t, f)

	w := doSarRequest(r, http.MethodGet, "/sar-declarations/not-a-uuid", nil)

	assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
	assert.Empty(t, f.Calls, "the usecase must not be reached with an invalid id")
}

func TestHandleDownloadExport_AttachmentHeaders(t *testing.T) {
	declId := uuid.New()
	exportId := uuid.New()
	xml := []byte(`<?xml version="1.0"?><declaration/>`)
	closed := false
	f := &fakeSarGenerationUsecase{Blob: models.Blob{
		FileName:   "export.xml",
		ReadCloser: sarTestBlobCloser{Reader: bytes.NewReader(xml), closed: &closed},
	}}
	r := newSarTestRouter(t, f)

	w := doSarRequest(r, http.MethodGet,
		"/sar-declarations/"+declId.String()+"/exports/"+exportId.String()+"/download", nil)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	disposition := w.Header().Get("Content-Disposition")
	assert.True(t, strings.HasPrefix(disposition, "attachment"), disposition)
	assert.Contains(t, disposition, declId.String()+"-"+exportId.String()+".xml")
	assert.True(t, strings.HasPrefix(w.Header().Get("Content-Type"), "application/xml"), w.Header().Get("Content-Type"))
	assert.Equal(t, xml, w.Body.Bytes())
	assert.True(t, closed, "the blob must be closed")

	calls := f.Called("DownloadExport")
	require.Len(t, calls, 1)
	assert.Equal(t, declId, calls[0].Args[0])
	assert.Equal(t, exportId, calls[0].Args[1])
}

func TestHandleGetDeclaration_ClearedValueFlag(t *testing.T) {
	id := uuid.New()
	st := sarHTTPState(id, uuid.New())
	st.Doc.Values[core.Path("a.cleared")] = core.Value{Value: "", Origin: core.OriginManual}
	st.Doc.Values[core.Path("a.filled")] = core.Value{Value: "x", Origin: core.OriginManual}
	f := &fakeSarGenerationUsecase{State: st}
	r := newSarTestRouter(t, f)

	w := doSarRequest(r, http.MethodGet, "/sar-declarations/"+id.String(), nil)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	body := sarHTTPDecode(t, w)
	doc, ok := body["document"].(map[string]any)
	require.True(t, ok)
	values, ok := doc["values"].(map[string]any)
	require.True(t, ok)
	cleared, ok := values["a.cleared"].(map[string]any)
	require.True(t, ok, "tombstone value must be exposed")
	assert.Equal(t, true, cleared["cleared"])
	filled, ok := values["a.filled"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, false, filled["cleared"])
}

func TestRoutes_NoGinConflict(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	require.NotPanics(t, func() {
		addRoutes(router, Configuration{}, usecases.Usecases{}, utils.Authentication{}, TokenHandler{}, slog.New(slog.DiscardHandler))
	})

	assert.True(t, hasRoute(router, http.MethodPost, "/sar-declarations/:id/exports"))
	assert.True(t, hasRoute(router, http.MethodGet, "/sar-declarations/:id/exports/:export_id/download"))
	assert.True(t, hasRoute(router, http.MethodGet, "/sar-formats/:format_key/spec"))
}
