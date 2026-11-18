package tracfin_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/usecases/sargen/core"
)

const noteAnalysis = "Le client Paul Martin, président de la société Alpha, a reçu des virements sans justificatif.\nAucune facture fournie."

func noteContext() core.StructuredContext {
	return core.StructuredContext{
		Selection: []core.ObjectRef{
			{Table: "clients", ObjectID: "c1"}, {Table: "accounts", ObjectID: "a.1"}, {Table: "transactions", ObjectID: "t1"},
		},
		AnalystText: noteAnalysis + "\nUn complément.",
	}
}

func validNote() map[string]any {
	obj := func(t, id string) map[string]any { return map[string]any{"table": t, "object_id": id} }
	return map[string]any{
		"summary": "story", "typology": "",
		"entities": []any{map[string]any{
			"object": obj("clients", "c1"), "role": "legal_representative", "description": "chairman",
			"basis": []any{map[string]any{"kind": "analysis", "reference": "président de la société Alpha"}},
		}},
		"links": []any{map[string]any{
			"from": obj("clients", "c1"), "to": obj("accounts", "a.1"), "nature": "holder",
			"basis": []any{map[string]any{"kind": "data", "reference": "accounts/a.1.client_id"}},
		}},
		"facts": []any{map[string]any{
			"statement": "no invoice", "objects": []any{obj("transactions", "t1")},
			"basis": []any{map[string]any{"kind": "analysis", "reference": "AUCUNE   facture\nfournie"}, map[string]any{"kind": "hypothesis", "reference": "hypothesis: the funds are loans"}},
		}},
		"suspicions": []any{
			map[string]any{"qualification": "unjustified transfers", "status": "asserted", "quote": "virements sans justificatif"},
			map[string]any{"qualification": "layering", "status": "hypothesis", "quote": ""},
		},
		"key_operations": []any{map[string]any{"objects": []any{obj("transactions", "t1")}, "description": "main transfer"}},
		"unknowns":       []any{map[string]any{"topic": "PPE status", "objects": []any{}, "why": "nothing says"}},
	}
}

func applyNote(t *testing.T, note any, sc core.StructuredContext) (*core.Document, error) {
	t.Helper()
	raw, err := json.Marshal(note)
	require.NoError(t, err)
	doc := planDoc(1, 2)
	before, _ := json.Marshal(doc)
	err = planFormat(t).ApplyStructured("understanding", doc, raw, sc)
	after, _ := json.Marshal(doc)
	assert.JSONEq(t, string(before), string(after), "the note never changes the document")
	return doc, err
}

func TestApplyUnderstanding_ValidNoteLeavesTheDocumentUnchanged(t *testing.T) {
	_, err := applyNote(t, validNote(), noteContext())
	require.NoError(t, err)
}

func cleanNote(t *testing.T, note any, sc core.StructuredContext) (map[string]any, []string, error) {
	t.Helper()
	raw, err := json.Marshal(note)
	require.NoError(t, err)
	cleaned, dropped, err := planFormat(t).(core.StructuredCleaner).CleanStructured("understanding", raw, sc)
	if err != nil {
		return nil, dropped, err
	}
	var out map[string]any
	require.NoError(t, json.Unmarshal(cleaned, &out))
	return out, dropped, nil
}

// The note is best-effort: an item whose references, quote or bounds fail is
// dropped and reported, the other items are kept.
func TestCleanUnderstanding_DropsInvalidItems(t *testing.T) {
	obj := func(t, id string) map[string]any { return map[string]any{"table": t, "object_id": id} }
	cases := map[string]struct {
		mutate func(n map[string]any)
		array  string // the array that loses its first item
		want   string
	}{
		"object outside the selection (entity)": {func(n map[string]any) {
			n["entities"].([]any)[0].(map[string]any)["object"] = obj("clients", "c9")
		}, "entities", "clients/c9 is not a selected object"},
		"object outside the selection (link)": {func(n map[string]any) {
			n["links"].([]any)[0].(map[string]any)["to"] = obj("accounts", "zz")
		}, "links", "accounts/zz is not a selected object"},
		"object outside the selection (key operation)": {func(n map[string]any) {
			n["key_operations"].([]any)[0].(map[string]any)["objects"] = []any{obj("transactions", "t9")}
		}, "key_operations", "transactions/t9"},
		"object outside the selection (unknown)": {func(n map[string]any) {
			n["unknowns"].([]any)[0].(map[string]any)["objects"] = []any{obj("x", "y")}
		}, "unknowns", "x/y"},
		"analysis basis not in the text": {func(n map[string]any) {
			n["entities"].([]any)[0].(map[string]any)["basis"] = []any{map[string]any{"kind": "analysis", "reference": "directeur général adjoint"}}
		}, "entities", "copied verbatim"},
		"analysis basis too short": {func(n map[string]any) {
			n["entities"].([]any)[0].(map[string]any)["basis"] = []any{map[string]any{"kind": "analysis", "reference": "le client"}}
		}, "entities", "at least 3 words"},
		"empty analysis basis": {func(n map[string]any) {
			n["entities"].([]any)[0].(map[string]any)["basis"] = []any{map[string]any{"kind": "analysis", "reference": "  "}}
		}, "entities", "copied verbatim"},
		"asserted suspicion quote not in the text": {func(n map[string]any) {
			n["suspicions"].([]any)[0].(map[string]any)["quote"] = "blanchiment très avéré"
		}, "suspicions", "asserted suspicion"},
		"asserted suspicion quote of one word": {func(n map[string]any) {
			n["suspicions"].([]any)[0].(map[string]any)["quote"] = "virements"
		}, "suspicions", "asserted suspicion"},
		"data basis without a field": {func(n map[string]any) {
			n["links"].([]any)[0].(map[string]any)["basis"] = []any{map[string]any{"kind": "data", "reference": "accounts/a.1."}}
		}, "links", "table/object_id.field"},
		"data basis on an object that is not selected": {func(n map[string]any) {
			n["links"].([]any)[0].(map[string]any)["basis"] = []any{map[string]any{"kind": "data", "reference": "accounts/a2.iban"}}
		}, "links", "table/object_id.field"},
		"data basis on a field absent from the snapshot": {func(n map[string]any) {
			n["links"].([]any)[0].(map[string]any)["basis"] = []any{map[string]any{"kind": "data", "reference": "accounts/a.1.nope"}}
		}, "links", "table/object_id.field"},
		"unknown basis kind": {func(n map[string]any) {
			n["facts"].([]any)[0].(map[string]any)["basis"] = []any{map[string]any{"kind": "guess", "reference": "x"}}
		}, "facts", "unknown value"},
		"unknown role": {func(n map[string]any) {
			n["entities"].([]any)[0].(map[string]any)["role"] = "boss"
		}, "entities", "role"},
		"unknown suspicion status": {func(n map[string]any) {
			n["suspicions"].([]any)[1].(map[string]any)["status"] = "maybe"
		}, "suspicions", "status"},
		"item string too long": {func(n map[string]any) {
			n["entities"].([]any)[0].(map[string]any)["description"] = strings.Repeat("a", 1001)
		}, "entities", "longer than 1000"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			n := validNote()
			c.mutate(n)
			sc := noteContext()
			sc.Snapshots = map[core.ObjectRef]map[string]any{
				{Table: "accounts", ObjectID: "a.1"}: {"client_id": "c1"},
			}
			cleaned, dropped, err := cleanNote(t, n, sc)
			require.NoError(t, err, "the note is not refused")
			require.Len(t, dropped, 1)
			assert.Contains(t, dropped[0], c.want)
			orig := validNote()[c.array].([]any)
			kept := cleaned[c.array].([]any)
			if c.array == "suspicions" && strings.Contains(name, "status") {
				assert.Len(t, kept, len(orig)-1)
				return
			}
			assert.Len(t, kept, len(orig)-1, "the invalid item is dropped")
			for k, v := range cleaned {
				if k != c.array {
					assert.Equal(t, validNote()[k], v, "the other parts are kept: %s", k)
				}
			}
		})
	}

	t.Run("a valid note is returned unchanged", func(t *testing.T) {
		cleaned, dropped, err := cleanNote(t, validNote(), noteContext())
		require.NoError(t, err)
		assert.Empty(t, dropped)
		assert.Equal(t, validNote()["facts"], cleaned["facts"])
	})
	t.Run("no selection known: every object item is dropped, the note stays", func(t *testing.T) {
		sc := noteContext()
		sc.Selection = nil
		cleaned, dropped, err := cleanNote(t, validNote(), sc)
		require.NoError(t, err)
		assert.NotEmpty(t, dropped)
		assert.Empty(t, cleaned["entities"])
		assert.Equal(t, "story", cleaned["summary"])
	})
}

// Only a note that cannot be read, or that exceeds the hard bounds, is refused.
func TestCleanUnderstanding_RefusesUnreadableOrOversized(t *testing.T) {
	for name, c := range map[string]struct {
		mutate func(n map[string]any)
		want   string
	}{
		"summary too long": {func(n map[string]any) { n["summary"] = strings.Repeat("é", 4001) }, "summary"},
		"string too long":  {func(n map[string]any) { n["typology"] = strings.Repeat("a", 1001) }, "typology"},
		"too many items": {func(n map[string]any) {
			items := make([]any, 301)
			for i := range items {
				items[i] = map[string]any{"topic": "t", "objects": []any{}, "why": "w"}
			}
			n["unknowns"] = items
		}, "unknowns: at most 300"},
	} {
		t.Run(name, func(t *testing.T) {
			n := validNote()
			c.mutate(n)
			_, err := applyNote(t, n, noteContext())
			require.Error(t, err)
			assert.Contains(t, err.Error(), c.want)
		})
	}
	t.Run("summary at the bound is accepted", func(t *testing.T) {
		n := validNote()
		n["summary"] = strings.Repeat("é", 4000)
		_, err := applyNote(t, n, noteContext())
		require.NoError(t, err)
	})
	t.Run("unreadable", func(t *testing.T) {
		err := planFormat(t).ApplyStructured("understanding", core.NewDocument(), json.RawMessage(`{"summary":3}`), noteContext())
		require.Error(t, err)
	})
	t.Run("invalid items do not refuse the note", func(t *testing.T) {
		n := validNote()
		n["entities"].([]any)[0].(map[string]any)["role"] = "boss"
		_, err := applyNote(t, n, noteContext())
		require.NoError(t, err)
	})
}

func TestUnderstandingSchema_IsStrictAndInTheSpec(t *testing.T) {
	specs := planFormat(t).Plan(core.PlanState{JobKind: string(models.SarJobKindGenerate), Doc: core.NewDocument()})
	require.Len(t, specs, 1)
	s := specs[0]
	assert.Equal(t, stepUnderstanding, s.Kind)
	assert.Equal(t, core.Path(""), s.TargetPath)
	assert.Empty(t, s.AllowedPaths)
	assert.Equal(t, core.TierHeavy, s.Tier)
	assert.False(t, s.FailJobOnLimit, "without a note the later steps run on the analysis alone")
	assert.True(t, s.NeedsCaseContext)
	assert.Equal(t, []string{"read_object", "follow_link", "field_spec", "search_code", "data_model"}, s.ToolNames)
	require.NotNil(t, s.OutputSchema)

	raw, err := json.Marshal(s.OutputSchema)
	require.NoError(t, err)
	var schema map[string]any
	require.NoError(t, json.Unmarshal(raw, &schema))
	assert.ElementsMatch(t, []any{"summary", "typology", "entities", "links", "facts", "suspicions", "key_operations", "unknowns"}, schema["required"])
	assert.Equal(t, false, schema["additionalProperties"])
	for _, want := range []string{"suspect", "probable_accomplice", "beneficiary", "originator", "counterparty", "possible_victim", "intermediary", "legal_representative", "other", "asserted", "hypothesis", "analysis", "data"} {
		assert.Contains(t, string(raw), `"`+want+`"`)
	}
}
