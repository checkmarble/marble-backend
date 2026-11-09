package api

import (
	"bytes"
	"context"
	"github.com/checkmarble/marble-backend/models"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"net/http/httptest"
	"testing"
)

type aiHTTPStub struct {
	called   string
	revision int64
}

func (s *aiHTTPStub) PrepareAI(_ context.Context, _ string, r int64) (models.TracfinAIRun, error) {
	s.called = "prepare"
	s.revision = r
	return models.TracfinAIRun{PromptSnapshot: map[string]string{"secret-prompt": "content"}}, nil
}
func (s *aiHTTPStub) GetAIRun(context.Context, string, string) (models.TracfinAIRun, error) {
	s.called = "get"
	return models.TracfinAIRun{}, nil
}
func (s *aiHTTPStub) ListAIHistory(context.Context, string) ([]models.TracfinAIRunSummary, error) {
	s.called = "history"
	return []models.TracfinAIRunSummary{{ID: "run", Kind: "correction", Feedback: "remarque"}}, nil
}
func (s *aiHTTPStub) SubmitAIAnalysis(_ context.Context, _, _ string, i models.TracfinAIInput) (models.TracfinDraft, error) {
	s.called = "analysis"
	s.revision = i.ExpectedRevision
	return models.TracfinDraft{}, nil
}
func (s *aiHTTPStub) StartAI(_ context.Context, _, _ string, r int64) (models.TracfinAIRun, error) {
	s.called = "start"
	s.revision = r
	return models.TracfinAIRun{}, nil
}
func (s *aiHTTPStub) ResumeAI(_ context.Context, _, _ string, r int64) (models.TracfinAIRun, error) {
	s.called = "resume"
	s.revision = r
	return models.TracfinAIRun{}, nil
}
func (s *aiHTTPStub) ApproveAI(_ context.Context, _, _ string, r int64) (models.TracfinDraft, error) {
	s.called = "approve"
	s.revision = r
	return models.TracfinDraft{}, nil
}
func TestTracfinAIRoutesDispatchStrictInputsAndHidePromptSnapshots(t *testing.T) {
	gin.SetMode(gin.TestMode)
	s := &aiHTTPStub{}
	router := gin.New()
	addTracfinAIRoutes(router, func(context.Context) tracfinAIHTTPUsecase { return s })
	prefix := "/cases/" + uuid.NewString() + "/tracfin/ai"
	for _, action := range []string{"prepare", "analysis", "start", "resume", "approve", "rework"} {
		path := prefix + "/runs/" + uuid.NewString() + "/" + action
		if action == "prepare" {
			path = prefix + "/prepare"
		}
		w := httptest.NewRecorder()
		router.ServeHTTP(w, httptest.NewRequest("POST", path, bytes.NewBufferString(`{"expected_revision":7}`)))
		require.Less(t, w.Code, 300)
		require.Equal(t, action, s.called)
		require.Equal(t, int64(7), s.revision)
		require.NotContains(t, w.Body.String(), "secret-prompt")
	}
	for _, body := range []string{`{"expected_revision":7,"unexpected":true}`, `{} {}`, `null`} {
		s.called = ""
		w := httptest.NewRecorder()
		router.ServeHTTP(w, httptest.NewRequest("POST", prefix+"/prepare", bytes.NewBufferString(body)))
		require.Equal(t, 400, w.Code)
		require.Empty(t, s.called)
	}
}

func (s *aiHTTPStub) ReworkAI(_ context.Context, _, _ string, i models.TracfinAIInput) (models.TracfinAIRun, error) {
	s.called = "rework"
	s.revision = i.ExpectedRevision
	return models.TracfinAIRun{}, nil
}

func TestTracfinAIHistoryRoute(t *testing.T) {
	gin.SetMode(gin.TestMode)
	s := &aiHTTPStub{}
	router := gin.New()
	addTracfinAIRoutes(router, func(context.Context) tracfinAIHTTPUsecase { return s })
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest("GET", "/cases/"+uuid.NewString()+"/tracfin/ai/history", nil))
	require.Equal(t, 200, w.Code)
	require.Equal(t, "history", s.called)
	require.Contains(t, w.Body.String(), `"runs":[{`)
	require.Contains(t, w.Body.String(), `"kind":"correction"`)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest("GET", "/cases/not-a-uuid/tracfin/ai/history", nil))
	require.Equal(t, 400, w.Code)
}
