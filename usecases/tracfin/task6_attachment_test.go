package tracfin

import (
	"bytes"
	"encoding/base64"
	"encoding/xml"
	"errors"
	"io"
	"testing"

	"github.com/checkmarble/marble-backend/models"
)

func TestTracfinAttachmentContentRoundTrip(t *testing.T) {
	c := task5Catalog(t)
	original := []byte{0, 1, 2, 0xff, '<', '&', '\n', 0x80, 42}
	meta := models.TracfinAttachment{SourceFileID: "file", CaseID: "case", BlobKey: "key", Size: int64(len(original))}
	piece := task5Block("piece", "/ds/pieceJointe/piece")
	piece.Attachment = &meta
	d := task5Draft(c, task5Block("attachments", "/ds/pieceJointe", piece))
	var out bytes.Buffer
	_, err := WriteXML(&out, d, []AttachmentContent{{BlockID: "piece", Reader: bytes.NewReader(original), Metadata: meta}}, c)
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Pieces []struct {
			Content string `xml:"contenu"` //nolint:tagliatelle // official TRACFIN element name
		} `xml:"pieceJointe>piece"`
	}
	if err := xml.Unmarshal(out.Bytes(), &doc); err != nil {
		t.Fatalf("XML: %v", err)
	}
	if len(doc.Pieces) != 1 {
		t.Fatalf("pieces: %d; XML: %s", len(doc.Pieces), out.String())
	}
	decoded, err := base64.StdEncoding.DecodeString(doc.Pieces[0].Content)
	if err != nil {
		t.Fatalf("base64: %v", err)
	}
	if !bytes.Equal(decoded, original) {
		t.Fatalf("decoded content %x != %x; XML: %s", decoded, original, out.String())
	}
}

type task6FailReader struct{}

func (task6FailReader) Read([]byte) (int, error) { return 0, errors.New("attachment read failed") }
func TestTracfinAttachmentReadError(t *testing.T) {
	c := task5Catalog(t)
	meta := models.TracfinAttachment{SourceFileID: "file", CaseID: "case", BlobKey: "key", Size: 1}
	piece := task5Block("piece", "/ds/pieceJointe/piece")
	piece.Attachment = &meta
	d := task5Draft(c, task5Block("attachments", "/ds/pieceJointe", piece))
	_, err := WriteXML(io.Discard, d, []AttachmentContent{{BlockID: "piece", Reader: task6FailReader{}, Metadata: meta}}, c)
	if err == nil {
		t.Fatal("attachment read error was ignored")
	}
}

func TestTracfinAttachmentLocalReference(t *testing.T) {
	c := task5Catalog(t)
	meta := models.TracfinAttachment{SourceFileID: "file", CaseID: "case", BlobKey: "key", Size: 1}
	piece := task5Block("piece", "/ds/pieceJointe/piece")
	piece.Attachment = &meta
	civil := task5Block("civil", "/ds/personnes/pp/etatCivil")
	civil.Fields["/ds/personnes/pp/etatCivil/idPieceJointe"] = []models.TracfinValue{task5Reference("link", "piece", "")}
	d := task5Draft(c, task5Block("people", "/ds/personnes", task5Block("person", "/ds/personnes/pp", civil)), task5Block("attachments", "/ds/pieceJointe", piece))
	var out bytes.Buffer
	_, err := WriteXML(&out, d, []AttachmentContent{{BlockID: "piece", Reader: bytes.NewReader([]byte("x")), Metadata: meta}}, c)
	if err != nil {
		t.Fatal(err)
	}
	var x struct {
		Piece struct {
			ID string `xml:"id,attr"`
		} `xml:"pieceJointe>piece"`
		Reference string `xml:"personnes>pp>etatCivil>idPieceJointe"`
	}
	if err := xml.Unmarshal(out.Bytes(), &x); err != nil {
		t.Fatal(err)
	}
	if x.Piece.ID == "" || x.Piece.ID != x.Reference {
		t.Fatalf("piece id and reference differ: %q, %q; %s", x.Piece.ID, x.Reference, out.String())
	}
	for _, ch := range x.Piece.ID {
		if ch < '0' || ch > '9' {
			t.Fatalf("piece ID is not numeric: %q", x.Piece.ID)
		}
	}
}

type task6ChunkReader struct {
	r   io.Reader
	max int
}

func (r task6ChunkReader) Read(p []byte) (int, error) {
	if len(p) > r.max {
		p = p[:r.max]
	}
	return r.r.Read(p)
}
func TestTracfinAttachmentXMLReaderChunkBoundaries(t *testing.T) {
	c := task5Catalog(t)
	original := bytes.Repeat([]byte{0, 255, 1, 2, 3}, 997)
	meta := models.TracfinAttachment{SourceFileID: "file", CaseID: "case", BlobKey: "key", Size: int64(len(original))}
	piece := task5Block("piece", "/ds/pieceJointe/piece")
	piece.Attachment = &meta
	d := task5Draft(c, task5Block("attachments", "/ds/pieceJointe", piece))
	var out bytes.Buffer
	_, err := WriteXML(&out, d, []AttachmentContent{{BlockID: "piece", Reader: task6ChunkReader{bytes.NewReader(original), 7}, Metadata: meta}}, c)
	if err != nil {
		t.Fatal(err)
	}
	v := ValidateXML(task6ChunkReader{bytes.NewReader(out.Bytes()), 11}, c)
	if v.Layers["xml_syntax"].Status == "failed" {
		t.Fatalf("chunked XML rejected: %+v", v.Diagnostics)
	}
	var doc struct {
		Content string `xml:"pieceJointe>piece>contenu"`
	}
	if err := xml.Unmarshal(out.Bytes(), &doc); err != nil {
		t.Fatal(err)
	}
	got, err := base64.StdEncoding.DecodeString(doc.Content)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, original) {
		t.Fatal("content changed across chunk boundaries")
	}
}

func TestTracfinAttachmentXMLReaderRejectsMalformedContent(t *testing.T) {
	c := task5Catalog(t)
	for _, xmlText := range []string{`<ds><pieceJointe><piece><contenu>%%%!</contenu></piece></pieceJointe></ds>`, `<ds><pieceJointe><piece><contenu>eA==`} {
		v := ValidateXML(task6ChunkReader{bytes.NewBufferString(xmlText), 1}, c)
		if v.Layers["xml_syntax"].Status != "failed" && v.Layers["internal_structure"].Status != "failed" {
			t.Fatalf("malformed attachment accepted: %s; %+v", xmlText, v)
		}
	}
}

func TestTracfinAttachmentXMLReaderRejectsEmbeddedMarkup(t *testing.T) {
	c := task5Catalog(t)
	for _, body := range []string{`<![CDATA[eA==]]>`, `eA==<!--comment-->`, `eA==<extra/>`} {
		input := `<ds><pieceJointe><piece id="1"><contenu>` + body + `</contenu></piece></pieceJointe></ds>`
		v := ValidateXML(task6ChunkReader{bytes.NewBufferString(input), 1}, c)
		if v.Layers["xml_syntax"].Status != "unavailable" || !task5HasCode(v.Diagnostics, "XML_ATTACHMENT_LEXICAL_FORM_UNSUPPORTED") {
			t.Fatalf("unsupported embedded markup not reported: %s: %+v", body, v)
		}
	}
}

func TestTracfinAttachmentReferenceCatalogTypes(t *testing.T) {
	c := task5Catalog(t)
	for _, id := range []string{"/ds/personnes/pp/etatCivil/idPieceJointe", "/ds/personnes/pp/notorieteDefavorable/idPieceJointe", "/ds/personnes/pp/idPieceJointeDonneesConnexion", "/ds/personnes/pm/activiteCoordonnee/idPieceJointe", "/ds/personnes/pm/dirigeant/autreDirigeantP/idPieceJointe", "/ds/personnes/pm/dirigeant/autreDirigeantM/idPieceJointe", "/ds/personnes/pm/infosFinancieresFiscales/idPieceJointe", "/ds/personnes/pm/notorieteDefavorable/idPieceJointe"} {
		field, ok := c.Fields[id]
		if !ok || field.ValueKind != "reference" {
			t.Errorf("%s: %+v", id, field)
		}
	}
	for _, id := range []string{"/ds/organisme/adresse/numero", "/ds/personnes/pp/coordonnees/adresse/numero", "/ds/personnes/pm/activiteCoordonnee/adresse/numero"} {
		field, ok := c.Fields[id]
		if !ok || field.ValueKind != "text" {
			t.Errorf("%s: %+v", id, field)
		}
	}
}
