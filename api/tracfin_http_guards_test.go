package api

import (
	"net/http"
	"strings"
	"testing"

	"github.com/checkmarble/marble-backend/models"
	"github.com/stretchr/testify/require"
)

func TestTracfinHTTPRejectsServerMetadataRecursively(t *testing.T) {
	path := "/cases/" + testCaseID + "/tracfin/draft"
	for _, injected := range []string{
		`"origin":{"source":"manual"}`,
		`"source_hash":"hash"`,
		`"attachment":{"source_file_id":"x","blob_key":"x"}`,
		`"bucket_name":"bucket"`,
		`"blob_key":"key"`,
		`"last_export":{}`,
		`"configuration":{"settings":{}}`,
	} {
		t.Run(injected, func(t *testing.T) {
			f := &tracfinHTTPFake{}
			body := `{"expected_revision":1,"changes":[{"action":"set_field","field":{"block_id":"b","field_id":"f"},"value":{"kind":"text",` + injected + `}}]}`
			w := tracfinRequest(f, http.MethodPatch, path, body)
			require.Equal(t, 400, w.Code, w.Body.String())
			require.Zero(t, f.calls)
		})
	}
}
func TestTracfinHTTPConfigurationRejectsFullDocumentAndInjectedProof(t *testing.T) {
	path := "/organizations/" + testCaseID + "/tracfin"
	for _, body := range []string{
		`{"settings":[],"mappings":[]}`,
		`{"expected_revision":0,"settings":[],"mappings":[],"document":{"root":{"children":[{"catalog_id":"/ds/personne"}]}}}`,
		`{"expected_revision":0,"settings":[{"field":{"block_id":"b","field_id":"f"},"value":{"kind":"text","origin":{"source":"settings"}}}],"mappings":[]}`,
		`{"expected_revision":0,"settings":[{"field":{"block_id":"b","field_id":"f"},"value":{"kind":"text","attachment":{"blob_key":"key"}}}],"mappings":[]}`,
		`{"expected_revision":0,"settings":[],"mappings":[],"selection":{}}`,
	} {
		t.Run(body, func(t *testing.T) {
			f := &tracfinHTTPFake{}
			w := tracfinRequest(f, http.MethodPut, path, body)
			require.Equal(t, 400, w.Code, w.Body.String())
			require.Zero(t, f.calls)
		})
	}
}
func TestTracfinHTTPStaleRevisions(t *testing.T) {
	for _, endpoint := range []string{"prefill", "validate", "export"} {
		t.Run(endpoint, func(t *testing.T) {
			f := &tracfinHTTPFake{err: models.ConflictError}
			w := tracfinRequest(f, http.MethodPost, "/cases/"+testCaseID+"/tracfin/"+endpoint, `{"expected_revision":7}`)
			require.Equal(t, 409, w.Code, w.Body.String())
			require.Equal(t, int64(7), f.revision)
		})
	}
}
func TestTracfinHTTPDownloadClosesReaderOnUsecaseError(t *testing.T) {
	reader := &tracfinReadCloser{Reader: strings.NewReader("unused")}
	f := &tracfinHTTPFake{reader: reader, err: models.ForbiddenError}
	w := tracfinRequest(f, http.MethodGet, "/cases/"+testCaseID+"/tracfin/exports/"+testExportID, "")
	require.Equal(t, 403, w.Code, w.Body.String())
	require.True(t, reader.closed)
}

func TestTracfinHTTPPieceSourceFileCommand(t *testing.T) {
	f := &tracfinHTTPFake{}
	w := tracfinRequest(f, http.MethodPatch, "/cases/"+testCaseID+"/tracfin/draft", `{"expected_revision":1,"changes":[{"action":"add_block","parent_block_id":"p","catalog_id":"/ds/pieceJointe/piece","source_file_id":"file-id"}]}`)
	require.Equal(t, 200, w.Code, w.Body.String())
	require.Len(t, f.input.Changes, 1)
	require.Equal(t, "file-id", f.input.Changes[0].SourceFileID)
}

func TestTracfinHTTPExportSizeLimit(t *testing.T) {
	f := &tracfinHTTPFake{err: models.TracfinSizeLimitError}
	w := tracfinRequest(f, http.MethodPost, "/cases/"+testCaseID+"/tracfin/export", `{"expected_revision":1}`)
	require.Equal(t, 413, w.Code, w.Body.String())
}
