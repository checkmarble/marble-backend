package api

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/usecases"
	"github.com/checkmarble/marble-backend/usecases/tracfin"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

const testCaseID = "11111111-1111-4111-8111-111111111111"
const testExportID = "22222222-2222-4222-8222-222222222222"

type tracfinHTTPFake struct {
	err      error
	calls    int
	limit    int
	revision int64
	reader   io.ReadSeekCloser
	size     int64
	input    models.TracfinDraftInput
	draft    models.TracfinDraft
	page     usecases.TracfinNavigationPage
}

func (f *tracfinHTTPFake) GetConfiguration(context.Context, uuid.UUID) (models.TracfinConfiguration, error) {
	f.calls++
	return models.TracfinConfiguration{}, f.err
}
func (f *tracfinHTTPFake) SaveConfiguration(_ context.Context, _ uuid.UUID, i models.TracfinConfigurationInput) (models.TracfinConfiguration, error) {
	f.calls++
	f.revision = i.ExpectedRevision
	return models.TracfinConfiguration{}, f.err
}
func (f *tracfinHTTPFake) GetContext(context.Context, string) (usecases.TracfinContext, error) {
	f.calls++
	return usecases.TracfinContext{}, f.err
}
func (f *tracfinHTTPFake) Navigate(_ context.Context, _ string, _ models.TracfinSelection, _ string, _ *string, limit int) (usecases.TracfinNavigationPage, error) {
	f.calls++
	f.limit = limit
	return f.page, f.err
}
func (f *tracfinHTTPFake) GetDraft(context.Context, string) (models.TracfinDraft, error) {
	f.calls++
	return f.draft, f.err
}
func (f *tracfinHTTPFake) CreateDraft(context.Context, string, bool) (models.TracfinDraft, error) {
	f.calls++
	return models.TracfinDraft{}, f.err
}
func (f *tracfinHTTPFake) UpdateDraft(_ context.Context, _ string, i models.TracfinDraftInput) (models.TracfinDraft, error) {
	f.calls++
	f.revision = i.ExpectedRevision
	f.input = i
	return models.TracfinDraft{}, f.err
}
func (f *tracfinHTTPFake) PrefillAtRevision(_ context.Context, _ string, r int64) (tracfin.PrefillResult, error) {
	f.calls++
	f.revision = r
	return tracfin.PrefillResult{}, f.err
}
func (f *tracfinHTTPFake) ValidateDraft(_ context.Context, _ string, r int64) (models.TracfinValidation, error) {
	f.calls++
	f.revision = r
	return models.TracfinValidation{Diagnostics: []models.TracfinDiagnostic{{Code: "MISSING", Layer: "business", Field: &models.TracfinFieldRef{BlockID: "block", FieldID: "field", ParentValueID: "owner"}}}}, f.err
}
func (f *tracfinHTTPFake) Export(_ context.Context, _ string, r int64) (models.TracfinExport, error) {
	f.calls++
	f.revision = r
	return models.TracfinExport{}, f.err
}
func (f *tracfinHTTPFake) DownloadExport(context.Context, string, string) (models.Blob, models.TracfinExport, error) {
	f.calls++
	return models.Blob{ReadCloser: f.reader, FileName: "../../unsafe.xml"}, models.TracfinExport{DraftRevision: 3, Size: f.size}, f.err
}
func tracfinRequest(f *tracfinHTTPFake, method, path, body string) *httptest.ResponseRecorder {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	addTracfinRoutes(router, func(context.Context) tracfinHTTPUsecase { return f })
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}
func TestTracfinHTTPErrorsAndDiagnostics(t *testing.T) {
	path := "/cases/" + testCaseID + "/tracfin/validate"
	for _, tc := range []struct {
		name   string
		err    error
		status int
	}{{"forbidden", models.ForbiddenError, 403}, {"not found", models.NotFoundError, 404}, {"stale", models.ConflictError, 409}, {"business diagnostics", nil, 200}} {
		t.Run(tc.name, func(t *testing.T) {
			f := &tracfinHTTPFake{err: tc.err}
			w := tracfinRequest(f, "POST", path, `{"expected_revision":2}`)
			require.Equal(t, tc.status, w.Code, w.Body.String())
			require.Equal(t, int64(2), f.revision)
			if tc.err == nil {
				require.Contains(t, w.Body.String(), "MISSING")
				require.Contains(t, w.Body.String(), `"parent_value_id":"owner"`)
				require.NotContains(t, w.Body.String(), `"value_id"`)
			}
		})
	}
}
func TestTracfinHTTPStrictPayloads(t *testing.T) {
	path := "/cases/" + testCaseID + "/tracfin/draft"
	for _, body := range []string{`{`, `null`, `{"expected_revision":1,"changes":[]} {}`, `{"changes":[]}`, `{"expected_revision":0,"changes":[]}`, `{"expected_revision":1,"changes":[],"last_export":{}}`, `{"expected_revision":1,"changes":[{"action":"set_field","field":{},"value":{"kind":"text","origin":{}}}]}`, `{"expected_revision":1,"changes":[{"action":"add_block","attachment":{"bucket_name":"x"}}]}`, `{"expected_revision":1,"changes":[{"action":"set_selection","selection":{"source_hash":"x"}}]}`, `{"expected_revision":1,"configuration":{}}`} {
		t.Run(body, func(t *testing.T) {
			f := &tracfinHTTPFake{}
			w := tracfinRequest(f, "PATCH", path, body)
			require.Equal(t, 400, w.Code, w.Body.String())
			require.Zero(t, f.calls)
		})
	}
	f := &tracfinHTTPFake{}
	w := tracfinRequest(f, "PATCH", path, `{"expected_revision":1,"changes":[],"padding":"`+strings.Repeat("x", 1024*1024)+`"}`)
	require.Equal(t, 413, w.Code, w.Body.String())
	require.Zero(t, f.calls)
}
func TestTracfinHTTPRevisionRequired(t *testing.T) {
	for _, endpoint := range []string{"prefill", "validate", "export"} {
		for _, body := range []string{`{}`, `{"expected_revision":0}`} {
			t.Run(endpoint+body, func(t *testing.T) {
				f := &tracfinHTTPFake{}
				w := tracfinRequest(f, "POST", "/cases/"+testCaseID+"/tracfin/"+endpoint, body)
				require.Equal(t, 400, w.Code, w.Body.String())
				require.Zero(t, f.calls)
			})
		}
	}
	f := &tracfinHTTPFake{}
	w := tracfinRequest(f, "PUT", "/organizations/"+testCaseID+"/tracfin", `{"expected_revision":0,"settings":[],"mappings":[]}`)
	require.Equal(t, 200, w.Code, w.Body.String())
	require.Equal(t, 1, f.calls)
}
func TestTracfinHTTPNavigateLimit(t *testing.T) {
	path := "/cases/" + testCaseID + "/tracfin/navigate"
	for _, tc := range []struct {
		body   string
		status int
		limit  int
	}{{`{"selection":{},"step_id":"s"}`, 200, 0}, {`{"selection":{},"step_id":"s","limit":1}`, 200, 1}, {`{"selection":{},"step_id":"s","limit":100}`, 200, 100}, {`{"selection":{},"step_id":"s","limit":0}`, 400, 0}, {`{"selection":{},"step_id":"s","limit":101}`, 400, 0}} {
		t.Run(tc.body, func(t *testing.T) {
			f := &tracfinHTTPFake{}
			w := tracfinRequest(f, "POST", path, tc.body)
			require.Equal(t, tc.status, w.Code, w.Body.String())
			if tc.status == 200 {
				if tc.limit == 0 {
					require.GreaterOrEqual(t, f.limit, 1)
					require.LessOrEqual(t, f.limit, 100)
				} else {
					require.Equal(t, tc.limit, f.limit)
				}
			} else {
				require.Zero(t, f.calls)
			}
		})
	}
}

type tracfinReadCloser struct {
	*strings.Reader
	closed bool
}

func (r *tracfinReadCloser) Close() error { r.closed = true; return nil }
func TestTracfinHTTPDownload(t *testing.T) {
	reader := &tracfinReadCloser{Reader: strings.NewReader("<ds>exact</ds>")}
	f := &tracfinHTTPFake{reader: reader, size: int64(len("<ds>exact</ds>"))}
	w := tracfinRequest(f, http.MethodGet, "/cases/"+testCaseID+"/tracfin/exports/"+testExportID, "")
	require.Equal(t, 200, w.Code, w.Body.String())
	require.Equal(t, "<ds>exact</ds>", w.Body.String())
	require.Contains(t, w.Header().Get("Content-Type"), "xml")
	require.Contains(t, w.Header().Get("Cache-Control"), "no-store")
	require.Contains(t, w.Header().Get("Content-Disposition"), ".xml")
	require.NotContains(t, w.Header().Get("Content-Disposition"), "../")
	require.True(t, reader.closed)
}
