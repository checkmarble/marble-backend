package api

import (
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/usecases/sargen/core"
)

func TestSarInputs_StrictBodies(t *testing.T) {
	id := uuid.New().String()
	many := func(n int) []map[string]string {
		out := make([]map[string]string, n)
		for i := range out {
			out[i] = map[string]string{"path": "a.b", "value": "x"}
		}
		return out
	}
	manyRefs := func(n int) []map[string]string {
		out := make([]map[string]string, n)
		for i := range out {
			out[i] = map[string]string{"table": "t", "object_id": "o"}
		}
		return out
	}
	tests := []struct {
		name, method, path string
		body               any
	}{
		{"missing version", http.MethodPost, "/sar-declarations/" + id + "/exports", map[string]any{}},
		{"zero version", http.MethodPost, "/sar-declarations/" + id + "/exports", map[string]any{"version": 0}},
		{"nil settings values", http.MethodPut, "/sar-format-settings/tracfin", map[string]any{}},
		{"missing option index", http.MethodPost, "/sar-questions/" + id + "/choose", map[string]any{"version": 1}},
		{"negative option index", http.MethodPost, "/sar-questions/" + id + "/choose", map[string]any{"version": 1, "option_index": -1}},
		{"too many fields", http.MethodPatch, "/sar-declarations/" + id + "/fields", map[string]any{"version": 1, "fields": many(501)}},
		{"too many adds", http.MethodPatch, "/sar-declarations/" + id + "/selection", map[string]any{"version": 1, "add": manyRefs(201)}},
		{"too many removes", http.MethodPatch, "/sar-declarations/" + id + "/selection", map[string]any{"version": 1, "remove": manyRefs(201)}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := &fakeSarGenerationUsecase{}
			w := doSarRequest(newSarTestRouter(t, f), tt.method, tt.path, tt.body)
			assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
			assert.Empty(t, f.Calls)
		})
	}
}

func TestSarInputs_OptionIndexZeroAccepted(t *testing.T) {
	f := &fakeSarGenerationUsecase{State: sarHTTPState(uuid.New(), uuid.New())}
	w := doSarRequest(newSarTestRouter(t, f), http.MethodPost, "/sar-questions/"+uuid.New().String()+"/choose",
		map[string]any{"version": 1, "option_index": 0})
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, 0, f.Called("ChooseQuestionOption")[0].Args[2])
}

func TestHandlePutSettings_ValidationRefusedListsIssues(t *testing.T) {
	f := &fakeSarGenerationUsecase{Issues: []core.Issue{{
		Path: "organisme.numIdPro", Code: "pattern", Message: "format invalide", Severity: core.SeverityBlocking,
	}}}
	f.Fail("PutOrgSettings", models.BadParameterError)
	w := doSarRequest(newSarTestRouter(t, f), http.MethodPut, "/sar-format-settings/tracfin",
		map[string]any{"values": map[string]string{"organisme.numIdPro": "x"}})

	require.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
	body := sarHTTPDecode(t, w)
	details, ok := body["details"].(map[string]any)
	require.True(t, ok, w.Body.String())
	issues, ok := details["issues"].([]any)
	require.True(t, ok)
	require.Len(t, issues, 1)
	issue := issues[0].(map[string]any)
	assert.Equal(t, "organisme.numIdPro", issue["path"])
	assert.Equal(t, "pattern", issue["code"])
	assert.Equal(t, "format invalide", issue["message"])
}
