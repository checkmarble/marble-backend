package api

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/usecases"
	"github.com/stretchr/testify/require"
)

func TestTracfinHTTPNavigateSnakeCaseObjects(t *testing.T) {
	cursor := "A1"
	f := &tracfinHTTPFake{page: usecases.TracfinNavigationPage{Objects: []models.DataModelObject{{Data: map[string]any{"object_id": "A1"}, Metadata: map[string]any{"valid_from": "2026-10-27T00:00:00Z"}}}, HasNextPage: true, NextCursor: &cursor}}
	w := tracfinRequest(f, "POST", "/cases/"+testCaseID+"/tracfin/navigate", `{"selection":{},"step_id":"s"}`)
	require.Equal(t, 200, w.Code, w.Body.String())
	var body map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	require.Equal(t, []any{map[string]any{"data": map[string]any{"object_id": "A1"}, "metadata": map[string]any{"valid_from": "2026-10-27T00:00:00Z"}}}, body["objects"])
	require.Equal(t, true, body["has_next_page"])
	require.Equal(t, "A1", body["next_cursor"])
	require.NotContains(t, w.Body.String(), `"Data"`)
}

func TestTracfinHTTPDraftChangesIdentifiers(t *testing.T) {
	path := "/cases/" + testCaseID + "/tracfin/draft"
	for name, body := range map[string]string{
		"empty change list":     `{"expected_revision":1,"changes":[]}`,
		"malformed file ID":     `{"expected_revision":1,"changes":[{"action":"add_block","block_id":"p","parent_block_id":"x","catalog_id":"/ds/pieceJointe/piece","source_file_id":"not-a-uuid"}]}`,
		"malformed decision ID": `{"expected_revision":1,"changes":[{"action":"set_selection","block_id":"op","selection":{"mapping_id":"m","root":{"id":"","table":""},"decision_id":"D1","historical_type":"Transfer"}}]}`,
	} {
		t.Run(name, func(t *testing.T) {
			f := &tracfinHTTPFake{}
			w := tracfinRequest(f, "PATCH", path, body)
			require.Equal(t, 400, w.Code, w.Body.String())
			require.Zero(t, f.calls, "refused before the usecase")
		})
	}
	f := &tracfinHTTPFake{}
	w := tracfinRequest(f, "PATCH", path, `{"expected_revision":1,"changes":[{"action":"set_selection","block_id":"op","selection":{"mapping_id":"m","root":{"id":"","table":""},"decision_id":"33333333-3333-4333-8333-333333333333","historical_type":"Transfer"}}]}`)
	require.Equal(t, 200, w.Code, w.Body.String())
}

func TestTracfinLongRequests(t *testing.T) {
	for _, tc := range []struct {
		method, route string
		long          bool
	}{
		{http.MethodPatch, "/cases/:case_id/tracfin/draft", true},
		{http.MethodPost, "/cases/:case_id/tracfin/export", true},
		{http.MethodGet, "/cases/:case_id/tracfin/exports/:export_id", true},
		{http.MethodPost, "/cases/:case_id/tracfin/prefill", false},
		{http.MethodGet, "/cases/:case_id/tracfin/draft", false},
		{http.MethodPut, "/organizations/:organization_id/tracfin", false},
	} {
		require.Equal(t, tc.long, tracfinLongRequest(tc.method, tc.route), tc.method+" "+tc.route)
	}
}
