package tracfin_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/usecases/sargen/core"
)

func TestPlan_PlacementDoneWithNothingPlacedIsNotRepeated(t *testing.T) {
	f := planFormat(t)
	waves, _ := planRun(t, f, core.PlanState{
		JobKind: string(models.SarJobKindGenerate),
		Doc:     core.NewDocument(),
		Done:    planPlacementDone(),
	})
	require.NotEmpty(t, waves)
	for _, w := range waves {
		assert.Empty(t, planWaveOf(w, stepPlacement), "the placement is not planned again")
		assert.Empty(t, planWaveOf(w, stepFillPerson))
		assert.Empty(t, planWaveOf(w, stepFillTransaction))
	}
	assert.Equal(t, []string{core.TaskKindReview}, planKinds(waves[len(waves)-1]))
}

func TestApplyPlacement_InvalidOutputsAreRefusedWithoutChange(t *testing.T) {
	f := planFormat(t)
	obj := `[{"table":"persons","object_id":"p-1"}]`
	cases := map[string]string{
		"unknown person kind":        `{"persons":[{"kind":"xx","objects":` + obj + `}],"financial_refs":[],"transactions":[]}`,
		"person without object":      `{"persons":[{"kind":"pp","objects":[]}],"financial_refs":[],"transactions":[]}`,
		"unknown ref kind":           `{"persons":[{"kind":"pp","objects":` + obj + `}],"financial_refs":[{"ref_kind":"Bitcoin","owners":[{"person_index":0,"lien":"titulaire"}],"objects":` + obj + `}],"transactions":[]}`,
		"bad person index":           `{"persons":[{"kind":"pp","objects":` + obj + `}],"financial_refs":[{"ref_kind":"Iban","owners":[{"person_index":3,"lien":"titulaire"}],"objects":` + obj + `}],"transactions":[]}`,
		"bad lien":                   `{"persons":[{"kind":"pp","objects":` + obj + `}],"financial_refs":[{"ref_kind":"Iban","owners":[{"person_index":0,"lien":"proprietaire"}],"objects":` + obj + `}],"transactions":[]}`,
		"no owner":                   `{"persons":[{"kind":"pp","objects":` + obj + `}],"financial_refs":[{"ref_kind":"Iban","owners":[],"objects":` + obj + `}],"transactions":[]}`,
		"transaction without object": `{"persons":[],"financial_refs":[],"transactions":[{"object":{"table":"","object_id":""}}]}`,
		"not json":                   `[`,
	}
	for name, raw := range cases {
		t.Run(name, func(t *testing.T) {
			doc := core.NewDocument()
			require.Error(t, f.ApplyStructured(stepPlacement, doc, json.RawMessage(raw)))
			assert.Empty(t, doc.Items, "a refused output leaves the document untouched")
		})
	}
	require.Error(t, f.ApplyStructured("unknown", core.NewDocument(), json.RawMessage(`{}`)))
}

func TestCompute_DerivesReferenceFromSharedSourceObject(t *testing.T) {
	f := compFormat(t)
	doc := core.NewDocument()
	person := doc.AddItem("personnes.pp", "pp")
	iban := doc.AddItem(person.Child("referencesFinancieres").Child("referenceFinanciereIban"), "referenceFinanciereIban")
	account := core.ObjectRef{Table: "accounts", ObjectID: "a-1"}
	doc.ItemSources[iban] = []core.ObjectRef{account}

	tx := compBase("100", "2026-03-01")
	tx.emetteurPersonne = string(person)
	item := addCompTx(doc, tx)
	doc.ItemSources[item] = []core.ObjectRef{account}

	res := f.Compute(doc)
	compApply(t, f, doc, res)

	assert.Equal(t, string(iban), doc.Values[item.Child("refEmission")].Value)
	assert.Equal(t, core.OriginComputed, doc.Values[item.Child("refEmission")].Origin)
	line := compLines(doc)[0]
	assert.Equal(t, string(iban), compLineValue(doc, line, "refFinanciereEmission"))

	// A manual reference is never replaced.
	doc.Values[item.Child("refEmission")] = core.Value{Value: "FR7612345987650123456789014", Origin: core.OriginManual}
	compApply(t, f, doc, f.Compute(doc))
	assert.Equal(t, "FR7612345987650123456789014", doc.Values[item.Child("refEmission")].Value)
}

func TestTools_OperationPreviewAndReadDocument(t *testing.T) {
	f := planFormat(t)
	doc := core.NewDocument()
	addCompTx(doc, compBase("100", "2026-03-01"))
	addCompTx(doc, compBase("50", "2026-03-02"))
	tools := f.Tools(core.ToolContext{Format: f, Doc: doc})
	require.Contains(t, tools, "tracfin_operation_preview")
	require.Contains(t, tools, "read_document")

	var preview struct {
		Lines []struct {
			Count       int    `json:"count"`
			TotalAmount string `json:"total_amount"`
		} `json:"lines"`
		TotalAmount string `json:"total_amount"`
		PeriodStart string `json:"period_start"`
	}
	raw := tools["tracfin_operation_preview"].Call(context.Background(), json.RawMessage(`{}`))
	require.NoError(t, json.Unmarshal([]byte(raw), &preview), raw)
	require.Len(t, preview.Lines, 1, raw)
	assert.Equal(t, 2, preview.Lines[0].Count)
	assert.Equal(t, "150", preview.Lines[0].TotalAmount)
	assert.Equal(t, "150", preview.TotalAmount)
	assert.Equal(t, "2026-03-01", preview.PeriodStart)
	assert.Empty(t, doc.Items["operations.operation"], "the preview never writes")

	out := tools["read_document"].Call(context.Background(), json.RawMessage(`{"prefix":"work.transactions[transactions-1]"}`))
	assert.Contains(t, out, "work.transactions[transactions-1].montant")
	assert.NotContains(t, out, "transactions-2")
	assert.Contains(t, tools["read_document"].Call(context.Background(), json.RawMessage(`{`)), "error")
}

func TestTools_OperationPreviewWithCandidateFillsAndBounds(t *testing.T) {
	f := planFormat(t)
	doc := core.NewDocument()
	item := doc.AddItem("work.transactions", "transactions") // empty: nothing to preview yet
	tools := f.Tools(core.ToolContext{Format: f, Doc: doc})
	call := func(args string) map[string]any {
		var out map[string]any
		require.NoError(t, json.Unmarshal([]byte(tools["tracfin_operation_preview"].Call(context.Background(), json.RawMessage(args))), &out))
		return out
	}
	assert.EqualValues(t, 0, call(`{}`)["line_count"])

	args, err := json.Marshal(map[string]any{"fills": []map[string]string{
		{"path": string(item.Child("montant")), "value": "12.5"},
		{"path": string(item.Child("devise")), "value": "EUR"},
		{"path": string(item.Child("typeOperation")), "value": "OT10"},
		{"path": "context.general.refInterne", "value": "x"}, // outside the transactions: refused
	}})
	require.NoError(t, err)
	out := call(string(args))
	assert.EqualValues(t, 1, out["line_count"])
	assert.Equal(t, "12.5", out["total_amount"])
	assert.NotEmpty(t, out["rejected_fills"])
	assert.Empty(t, doc.Values, "the preview works on a copy")

	// Bounded: many groups list at most 30 lines and many transactions at most 10 per line.
	big := core.NewDocument()
	for i := 0; i < 40; i++ {
		addCompTx(big, compTx{montant: "1", date: "2026-03-01", devise: "EUR", typeOperation: "OT10", emetteurLibre: string(rune('A'+i%26)) + string(rune('a'+i/26))})
	}
	for i := 0; i < 15; i++ {
		addCompTx(big, compBase("1", "2026-03-02"))
	}
	bigOut := f.Tools(core.ToolContext{Format: f, Doc: big})["tracfin_operation_preview"].Call(context.Background(), json.RawMessage(`{}`))
	var parsed struct {
		Lines []struct {
			Transactions []string `json:"transactions"`
		} `json:"lines"`
		LineCount int    `json:"line_count"`
		Warning   string `json:"warning"`
	}
	require.NoError(t, json.Unmarshal([]byte(bigOut), &parsed))
	assert.Len(t, parsed.Lines, 30)
	assert.Equal(t, 41, parsed.LineCount)
	assert.NotEmpty(t, parsed.Warning)
	for _, l := range parsed.Lines {
		assert.LessOrEqual(t, len(l.Transactions), 10)
	}
}
