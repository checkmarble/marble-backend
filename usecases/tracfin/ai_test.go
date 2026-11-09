package tracfin

import (
	"encoding/json"
	"github.com/checkmarble/marble-backend/models"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestAIProposalProtectsAnalysisAndManualValues(t *testing.T) {
	c, err := LoadCatalog()
	require.NoError(t, err)
	text := "Texte humain <exact>\n"
	d := models.TracfinDraft{Document: models.TracfinDocument{Root: models.TracfinBlock{ID: "root", CatalogID: "/ds", Fields: map[string][]models.TracfinValue{"/ds/analyse": {{ID: "analysis", Kind: "text", Text: &text, Origin: &models.TracfinOrigin{Source: "manual"}}}}}}}
	p := AISectionProposal{Values: []AIValue{{Key: "v", BlockKey: "root", FieldID: "/ds/analyse", Kind: "text", Text: &text, Explanation: "copy", Sources: []AIEvidence{{SourceID: "s", Locator: "text"}}}}}
	_, diags := ApplyAISection(d, "declaration", p, map[string]bool{"s": true}, c, "run")
	require.NotEmpty(t, diags)
	require.Equal(t, text, *d.Document.Root.Fields["/ds/analyse"][0].Text)
}
func TestAIProposalRequiresEvidenceAndRejectsForeignSource(t *testing.T) {
	c := task5Catalog(t)
	d := task5Draft(c, task5Civil(nil))
	text := "Smith"
	p := AISectionProposal{Values: []AIValue{{Key: "v", BlockKey: "civil", FieldID: "/ds/personnes/pp/etatCivil/nomNaissance", Kind: "text", Text: &text, Explanation: "from decision", Sources: []AIEvidence{{SourceID: "foreign", Locator: "/name"}}}}}
	_, ds := ApplyAISection(d, "persons", p, map[string]bool{"decision:1": true}, c, "run")
	require.NotEmpty(t, ds)
	require.Equal(t, "EVIDENCE_REQUIRED", ds[0].Code)
}

func TestAIAttachmentProposalPreservesBlockProvenance(t *testing.T) {
	c := task5Catalog(t)
	d := task5Draft(c, task5Block("attachments", "/ds/pieceJointe"))
	p := AISectionProposal{Blocks: []AIBlock{{Key: "new-piece", ParentKey: "attachments", CatalogID: "/ds/pieceJointe/piece"}}, Attachments: []AIAttachment{{BlockKey: "new-piece", SourceID: "file:1", Explanation: "case document", Sources: []AIEvidence{{SourceID: "file:1", Locator: "/text"}}}}}
	out, ds := ApplyAISection(d, "attachments", p, map[string]bool{"file:1": true}, c, "run")
	for _, diag := range ds {
		require.NotEqual(t, "error", diag.Severity, "%+v", ds)
	}
	piece := out.Document.Root.Children[0].Children[0]
	require.NotNil(t, piece.Origin)
	require.Equal(t, "new-piece", piece.Origin.SourceFingerprint)
	require.Equal(t, "run", piece.Origin.RunID)
	require.Equal(t, "case document", piece.Origin.Explanation)
	require.Equal(t, p.Attachments[0].Sources, piece.Origin.Evidence)
}

func TestAIProposalStrictJSON(t *testing.T) {
	var p AISectionProposal
	require.Error(t, DecodeAIJSON([]byte(`{"blocks":[],"unknown":true}`), &p))
	require.Error(t, DecodeAIJSON([]byte(`{} {}`), &p))
	require.NoError(t, DecodeAIJSON(json.RawMessage(`{"blocks":[],"values":[],"attachments":[],"missing":[],"conflicts":[]}`), &p))
}

func TestAIProposalEvidenceLocators(t *testing.T) {
	sources := []models.TracfinAISource{{ID: "decision:1", Locator: "1", Content: json.RawMessage(`{"amount":123,"note":"A factual statement"}`)}}
	for _, tc := range []struct {
		id, locator string
		valid       bool
	}{
		{"decision:1", "/amount", true}, {"decision:1", "$.amount", true}, {"decision:1", "A factual statement", true},
		{"decision:1", "/invented", false}, {"foreign", "/amount", false},
		{"analysis:a", "text", true}, {"analysis:a", "specific phrase", true}, {"analysis:a", "invented", false},
	} {
		p := AISectionProposal{Values: []AIValue{{Sources: []AIEvidence{{SourceID: tc.id, Locator: tc.locator}}}}}
		ds := ValidateAIProposalEvidence(p, sources, "a", "some specific phrase here")
		require.Equal(t, tc.valid, len(ds) == 0, "%s %s: %+v", tc.id, tc.locator, ds)
	}
}

func TestAIAttachmentProposalChecks(t *testing.T) {
	c := task5Catalog(t)
	piece := task5Block("piece", "/ds/pieceJointe/piece")
	d := task5Draft(c, task5Block("attachments", "/ds/pieceJointe", piece))
	good := AIAttachment{BlockKey: "piece", SourceID: "file:1", Explanation: "from case", Sources: []AIEvidence{{SourceID: "file:1", Locator: "/text"}}}
	for _, tc := range []struct {
		name, section string
		attachment    AIAttachment
		sources       map[string]bool
		code          string
	}{
		{"wrong section", "persons", good, map[string]bool{"file:1": true}, "SECTION_SCOPE"},
		{"foreign file", "attachments", good, map[string]bool{}, "ATTACHMENT_SOURCE"},
		{"wrong target", "attachments", AIAttachment{BlockKey: "root", SourceID: "file:1", Explanation: good.Explanation, Sources: good.Sources}, map[string]bool{"file:1": true}, "ATTACHMENT_TARGET"},
		{"non-file source", "attachments", AIAttachment{BlockKey: "piece", SourceID: "event:1", Explanation: good.Explanation, Sources: good.Sources}, map[string]bool{"event:1": true}, "ATTACHMENT_SOURCE"},
		{"missing evidence", "attachments", AIAttachment{BlockKey: "piece", SourceID: "file:1"}, map[string]bool{"file:1": true}, "EVIDENCE_REQUIRED"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, ds := ApplyAISection(d, tc.section, AISectionProposal{Attachments: []AIAttachment{tc.attachment}}, tc.sources, c, "run")
			require.NotEmpty(t, ds)
			require.Equal(t, tc.code, ds[0].Code)
		})
	}
}

func TestAIProposalKeepsConditionalBusinessErrors(t *testing.T) {
	c := task5Catalog(t)
	piece := task5Block("piece", "/ds/pieceJointe/piece")
	piece.Fields["/ds/pieceJointe/piece/type"] = []models.TracfinValue{task5Text("type", "TA2")}
	d := task5Draft(c, task5Block("attachments", "/ds/pieceJointe", piece))
	_, ds := ApplyAISection(d, "attachments", AISectionProposal{}, nil, c, "run")
	require.True(t, task5HasCode(ds, "IDENTITY_PIECE_DETAILS_REQUIRED"), "%+v", ds)
}

func TestAIProposalMissingRequiredIsPartial(t *testing.T) {
	c := task5Catalog(t)
	d := task5Draft(c, task5Block("people", "/ds/personnes", task5Block("person", "/ds/personnes/pp", task5Block("civil", "/ds/personnes/pp/etatCivil"))))
	_, ds := ApplyAISection(d, "persons", AISectionProposal{}, nil, c, "run")
	require.True(t, task5HasCode(ds, "AI_MISSING_REQUIRED"), "%+v", ds)
	for _, diag := range ds {
		if diag.Code == "AI_MISSING_REQUIRED" {
			require.Equal(t, "warning", diag.Severity)
		}
	}
}

func TestAIJSONRejectsNullAndMissingRequiredArrays(t *testing.T) {
	for _, raw := range []string{`null`, `{}`, `{"findings":null}`} {
		var review AIReview
		require.Error(t, DecodeAIJSON([]byte(raw), &review))
	}
	var review AIReview
	require.NoError(t, DecodeAIJSON([]byte(`{"findings":[]}`), &review))
}

func TestAIAttributeCanReferenceRetainedManualScalar(t *testing.T) {
	c := task5Catalog(t)
	civil := task5Civil(map[string][]models.TracfinValue{"/ds/personnes/pp/etatCivil/nationalite": {task5Text("human-country", "FR")}, "/ds/personnes/pp/etatCivil/nomNaissance": {task5Text("name", "Dupont")}, "/ds/personnes/pp/etatCivil/prenom": {task5Text("first", "Jean")}})
	d := task5Draft(c, civil)
	text := "code"
	p := AISectionProposal{Values: []AIValue{{Key: "type", BlockKey: "civil", FieldID: "/ds/personnes/pp/etatCivil/nationalite/@type", ParentValueKey: "human-country", Kind: "text", Text: &text, Explanation: "confirmed nationality", Sources: []AIEvidence{{SourceID: "s", Locator: "text"}}}}}
	out, diags := ApplyAISection(d, "persons", p, map[string]bool{"s": true}, c, "run")
	for _, diag := range diags {
		require.NotEqual(t, "error", diag.Severity, "%+v", diags)
	}
	var attr []models.TracfinValue
	WalkBlocks(out.Document.Root, func(b models.TracfinBlock) {
		if b.ID == "civil" {
			attr = b.Fields["/ds/personnes/pp/etatCivil/nationalite/@type"]
		}
	})
	require.Len(t, attr, 1)
	require.Equal(t, "human-country", attr[0].ParentValueID)
}
