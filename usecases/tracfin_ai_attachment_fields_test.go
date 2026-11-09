package usecases

import (
	"testing"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/usecases/tracfin"
	"github.com/stretchr/testify/require"
)

func TestAIAttachmentNameAndMimeComeFromTheFile(t *testing.T) {
	c, err := tracfin.LoadCatalog()
	require.NoError(t, err)
	piece := models.TracfinBlock{ID: "piece", CatalogID: "/ds/pieceJointe/piece", Fields: map[string][]models.TracfinValue{}}
	d := models.TracfinDraft{Document: models.TracfinDocument{Root: models.TracfinBlock{ID: "root", CatalogID: "/ds", Children: []models.TracfinBlock{{ID: "pj", CatalogID: "/ds/pieceJointe", Children: []models.TracfinBlock{piece}}}}}}
	source := models.TracfinAISource{ID: "file:f1", Kind: "attachment", Content: []byte(`{"id":"f1","name":"releve.pdf","mime":"application/pdf"}`)}
	out, ds := aiSetAttachmentFileFields(d, c, "piece", source, "run")
	require.Empty(t, ds)
	fields := out.Document.Root.Children[0].Children[0].Fields
	require.Equal(t, "releve.pdf", *fields["/ds/pieceJointe/piece/nom"][0].Text)
	require.Equal(t, "application/pdf", *fields["/ds/pieceJointe/piece/type_mime"][0].Text)
	require.Equal(t, "calculated", fields["/ds/pieceJointe/piece/type_mime"][0].Origin.Source)
	require.Equal(t, []models.TracfinAIEvidence{{SourceID: "file:f1", Locator: "/mime"}}, fields["/ds/pieceJointe/piece/type_mime"][0].Origin.Evidence)

	png := models.TracfinAISource{ID: "file:f2", Kind: "attachment", Content: []byte(`{"id":"f2","name":"passeport.png","mime":"image/png"}`)}
	out, ds = aiSetAttachmentFileFields(d, c, "piece", png, "run")
	fields = out.Document.Root.Children[0].Children[0].Fields
	require.Equal(t, "passeport.png", *fields["/ds/pieceJointe/piece/nom"][0].Text)
	require.Empty(t, fields["/ds/pieceJointe/piece/type_mime"], "a MIME type outside the allowed list is never written")
	require.Len(t, ds, 1)
	require.Equal(t, "ATTACHMENT_MIME_UNSUPPORTED", ds[0].Code)
	require.Equal(t, "warning", ds[0].Severity)
	require.Contains(t, ds[0].Message, "image/png")
}

func TestAIContextDetectsFileMime(t *testing.T) {
	require.Equal(t, "application/pdf", aiDetectMime([]byte("%PDF-1.4\n...")))
	require.Equal(t, "image/png", aiDetectMime([]byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR")))
	require.Equal(t, "text/plain", aiDetectMime([]byte("hello")))
}

func TestManualAttachmentSelectionFillsNameAndMimeFromContent(t *testing.T) {
	ctx, uc, repo, _, blobs := task6Setup(t)
	blobs.data["source"] = []byte("%PDF-1.4\n1 0 obj\n")
	d, err := task6Select(t, ctx, uc, repo)
	require.NoError(t, err)
	require.Equal(t, "application/pdf", *findDraftField(d.Document.Root, "/ds/pieceJointe/piece/type_mime")[0].Text)
	require.Equal(t, "document.pdf", *findDraftField(d.Document.Root, "/ds/pieceJointe/piece/nom")[0].Text)
}
