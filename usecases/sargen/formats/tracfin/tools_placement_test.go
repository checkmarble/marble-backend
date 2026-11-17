package tracfin_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/usecases/sargen/core"
	"github.com/checkmarble/marble-backend/usecases/sargen/formats/tracfin"
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

func TestPlan_JudgmentAndReviewAskForCaseContext(t *testing.T) {
	f := planFormat(t)
	waves, _ := planRun(t, f, core.PlanState{
		JobKind: string(models.SarJobKindGenerate), Doc: planDoc(1, 1), Done: planPlacementDone(),
	})
	for _, w := range waves {
		for _, s := range w {
			want := s.Kind == stepJudgment || s.Kind == core.TaskKindReview
			assert.Equal(t, want, s.NeedsCaseContext, s.Kind)
		}
	}
}

func TestCompute_ExactDecimalsAndStrictAmounts(t *testing.T) {
	f := compFormat(t)
	doc := core.NewDocument()
	addCompTx(doc, compBase("0.1", "2026-03-01"))
	addCompTx(doc, compBase("0.2", "2026-03-02"))
	addCompTx(doc, compBase("1.125", "2026-03-03"))
	addCompTx(doc, compBase("1/2", "2026-03-04")) // not a decimal: ignored
	addCompTx(doc, compBase("1e3", "2026-03-04"))
	addCompTx(doc, compBase("5", "2026-02-30")) // counted, but its date is not a real day
	compApply(t, f, doc, f.Compute(doc))

	assert.Equal(t, "6.425", doc.Values["operations.montantTotal"].Value)
	assert.Equal(t, "2026-03-01", doc.Values["operations.dateDebut"].Value)
	assert.Equal(t, "2026-03-03", doc.Values["operations.dateFin"].Value)

	only := core.NewDocument()
	addCompTx(only, compBase("0.1", "2026-03-01"))
	addCompTx(only, compBase("0.2", "2026-03-02"))
	compApply(t, f, only, f.Compute(only))
	assert.Equal(t, "0.3", only.Values["operations.montantTotal"].Value)
}

func TestApplyPlacement_DuplicatesAreCreatedOnce(t *testing.T) {
	f := planFormat(t)
	doc := core.NewDocument()
	out := `{"persons":[{"kind":"pp","objects":[{"table":"persons","object_id":"p-1"}]}],
	 "financial_refs":[{"ref_kind":"Iban","owners":[{"person_index":0,"lien":"titulaire"},{"person_index":0,"lien":"titulaire"}],"objects":[{"table":"accounts","object_id":"a-1"}]}],
	 "transactions":[{"object":{"table":"transactions","object_id":"t-1"}},{"object":{"table":"transactions","object_id":"t-1"}}]}`
	require.NoError(t, f.ApplyStructured(stepPlacement, doc, json.RawMessage(out)))
	assert.Len(t, doc.Items["work.transactions"], 1)
	assert.Len(t, doc.Items["personnes.pp[pp-1].referencesFinancieres.referenceFinanciereIban"], 1)
}

func TestApplyPlacement_ReferenceWithoutValidOwnerIsDroppedAndReported(t *testing.T) {
	f := planFormat(t)
	obj := `{"table":"accounts","object_id":"a-9"}`
	person := `{"kind":"pp","objects":[{"table":"persons","object_id":"p-1"}]}`
	for name, owners := range map[string]string{
		"no owner":         `[]`,
		"bad person index": `[{"person_index":3,"lien":"titulaire"}]`,
		"bad lien":         `[{"person_index":0,"lien":"proprietaire"}]`,
	} {
		t.Run(name, func(t *testing.T) {
			raw := json.RawMessage(`{"persons":[` + person + `],"financial_refs":[
				{"ref_kind":"Iban","owners":` + owners + `,"objects":[` + obj + `]},
				{"ref_kind":"Iban","owners":[{"person_index":0,"lien":"titulaire"}],"objects":[{"table":"accounts","object_id":"a-1"}]}],
				"transactions":[]}`)
			doc := core.NewDocument()
			require.NoError(t, f.ApplyStructured(stepPlacement, doc, raw))
			assert.Len(t, doc.Items["personnes.pp[pp-1].referencesFinancieres.referenceFinanciereIban"], 1, "the valid reference is kept")

			sq, ok := f.(core.StructuredQuestioner)
			require.True(t, ok)
			qs := sq.StructuredQuestions(stepPlacement, raw)
			require.Len(t, qs, 1)
			assert.Equal(t, "référence financière sans titulaire déclaré : accounts/a-9", qs[0].Text)
		})
	}
}

func TestApplyPlacement_TransactionSourcesIncludeLinkedAccount(t *testing.T) {
	f := planFormat(t)
	doc := core.NewDocument()
	raw := json.RawMessage(`{"persons":[],"financial_refs":[],"transactions":[
		{"objects":[{"table":"transactions","object_id":"t-1"},{"table":"accounts","object_id":"a-1"}]}]}`)
	require.NoError(t, f.ApplyStructured(stepPlacement, doc, raw))
	tx := core.Path("work.transactions[" + doc.Items["work.transactions"][0] + "]")
	assert.Equal(t, []core.ObjectRef{{Table: "transactions", ObjectID: "t-1"}, {Table: "accounts", ObjectID: "a-1"}}, doc.ItemSources[tx])
}

// derivedDoc is a document with a person holding an IBAN reference, and a
// transaction sharing the account object with that reference.
func derivedDoc() (doc *core.Document, tx, iban core.Path) {
	doc = core.NewDocument()
	person := doc.AddItem("personnes.pp", "pp")
	iban = doc.AddItem(person.Child("referencesFinancieres").Child("referenceFinanciereIban"), "referenceFinanciereIban")
	doc.Values[iban.Child("iban")] = core.Value{Value: "DE89370400440532013000", Origin: core.OriginLLM}
	account := core.ObjectRef{Table: "accounts", ObjectID: "a-1"}
	doc.ItemSources[iban] = []core.ObjectRef{account}
	b := compBase("100", "2026-03-01")
	b.paysOrigine, b.paysArrivee = "", ""
	b.emetteurPersonne = string(person)
	tx = addCompTx(doc, b)
	doc.ItemSources[tx] = []core.ObjectRef{{Table: "transactions", ObjectID: "t-1"}, account}
	return doc, tx, iban
}

func TestCompute_DerivesReferenceAndCountryFromTheIban(t *testing.T) {
	f := compFormat(t)
	doc, tx, iban := derivedDoc()
	compApply(t, f, doc, f.Compute(doc))

	assert.Equal(t, string(iban), doc.Values[tx.Child("refEmission")].Value)
	assert.Equal(t, "DE", doc.Values[tx.Child("paysOrigine")].Value)
	assert.Equal(t, core.OriginComputed, doc.Values[tx.Child("paysOrigine")].Origin)
	assert.Equal(t, "DE", compLineValue(doc, compLines(doc)[0], "paysOrigineFonds"))
}

func TestCompute_DerivedValuesYieldToManualLLMAndCleared(t *testing.T) {
	f := compFormat(t)
	doc, tx, _ := derivedDoc()
	doc.Values[tx.Child("paysOrigine")] = core.Value{Value: "FR", Origin: core.OriginLLM}
	compApply(t, f, doc, f.Compute(doc))
	assert.Equal(t, "FR", doc.Values[tx.Child("paysOrigine")].Value, "a value written by the model is never replaced")

	// The analyst clears the derived reference: it stays cleared.
	core.ApplyFills(f.Spec(), doc, []core.Fill{{Path: tx.Child("refEmission"), Value: "", Origin: core.OriginManual}},
		core.ApplyOptions{Overwrite: core.OverwriteAll})
	compApply(t, f, doc, f.Compute(doc))
	assert.Equal(t, "", doc.Values[tx.Child("refEmission")].Value)
	assert.Equal(t, core.OriginManual, doc.Values[tx.Child("refEmission")].Origin)
	assert.Equal(t, "", compLineValue(doc, compLines(doc)[0], "refFinanciereEmission"))
}

func TestCompute_StaleDerivedValueIsRecomputedAndCleared(t *testing.T) {
	f := compFormat(t)
	doc, tx, iban := derivedDoc()
	compApply(t, f, doc, f.Compute(doc))
	require.Equal(t, "DE", doc.Values[tx.Child("paysOrigine")].Value)

	doc.Values[iban.Child("iban")] = core.Value{Value: "FR7630006000011234567890189", Origin: core.OriginManual}
	compApply(t, f, doc, f.Compute(doc))
	assert.Equal(t, "FR", doc.Values[tx.Child("paysOrigine")].Value)

	doc.Values[iban.Child("iban")] = core.Value{Value: "1234", Origin: core.OriginManual} // not an IBAN
	compApply(t, f, doc, f.Compute(doc))
	_, has := doc.Values[tx.Child("paysOrigine")]
	assert.False(t, has, "a derived value that no longer derives goes away")
}

func TestCompute_StalePeriodGoesAwayAndEmptyCurrencyIsExcluded(t *testing.T) {
	f := compFormat(t)
	doc := core.NewDocument()
	item := addCompTx(doc, compBase(".5", "2026-03-01"))
	compApply(t, f, doc, f.Compute(doc))
	assert.Equal(t, "0.5", doc.Values["operations.montantTotal"].Value, "a leading-dot amount is read like the montant pattern")
	assert.Equal(t, "2026-03-01", doc.Values["operations.dateDebut"].Value)

	doc.Values[item.Child("date")] = core.Value{Value: "2026-02-30", Origin: core.OriginManual}
	compApply(t, f, doc, f.Compute(doc))
	_, has := doc.Values["operations.dateDebut"]
	assert.False(t, has, "no valid date: the computed period is removed")

	delete(doc.Values, item.Child("devise"))
	res := f.Compute(doc)
	compApply(t, f, doc, res)
	assert.Empty(t, compLines(doc), "an empty currency is not EUR")
}

func TestPlan_TransactionAndCorrectionTasksSeeDeclaredPersons(t *testing.T) {
	f := planFormat(t)
	doc := core.NewDocument()
	person := doc.AddItem("personnes.pp", "pp")
	iban := doc.AddItem(person.Child("referencesFinancieres").Child("referenceFinanciereIban"), "referenceFinanciereIban")
	doc.ItemSources[person] = []core.ObjectRef{{Table: "clients", ObjectID: "c1"}}
	doc.ItemSources[iban] = []core.ObjectRef{{Table: "accounts", ObjectID: "a1"}}
	doc.AddItem("work.transactions", "transactions")

	specs := f.Plan(core.PlanState{JobKind: string(models.SarJobKindGenerate), Doc: doc, Done: planPlacementDone()})
	tx := planWaveOf(planBatch(specs), stepFillTransaction)
	require.Len(t, tx, 1)
	assert.Contains(t, tx[0].ToolNames, "read_document")
	assert.Contains(t, tx[0].ToolNames, "tracfin_operation_preview")
	block, ok := tx[0].Payload[tracfin.PayloadDeclared].([]map[string]any)
	require.True(t, ok)
	require.Len(t, block, 2)
	assert.Equal(t, string(person), block[0]["path"])
	assert.Equal(t, "pp", block[0]["kind"])
	assert.Equal(t, string(iban), block[1]["path"])
	assert.Equal(t, "Iban", block[1]["kind"])
	assert.Equal(t, string(person), block[1]["owner"])

	// A person's fill task may fill its declared references (not their lien), never add one.
	p := planWaveOf(planBatch(specs), stepFillPerson)[0]
	assert.True(t, planAllows(p, iban.Child("iban")))
	assert.False(t, planAllows(p, iban.Child("lien")))
	assert.False(t, planAllows(p, person.Child("referencesFinancieres").Child("referenceFinanciereIban[+x]").Child("iban")))
	assert.False(t, planAllows(p, person.Child("referencesFinancieres").Child("referenceFinanciereAutre[+y]").Child("NumeroReference")))
	assert.True(t, planAllows(p, person.Child("etatCivil").Child("nomNaissance")))
}

func TestPlan_ComputeAfterCorrectionWaitsForTheCorrections(t *testing.T) {
	f := planFormat(t)
	st := planUntil(t, f, core.PlanState{
		JobKind: string(models.SarJobKindGenerate), Doc: planDoc(1, 0), Done: planPlacementDone(),
	}, core.TaskKindReview)
	st.Issues = []core.Issue{{Path: "personnes.pp[pp-1].etatCivil.dateNaissance", Code: core.CodeBadDate, Severity: core.SeverityBlocking, Message: "m"}}
	wave := f.Plan(st)
	st.Done = append(st.Done, planDone(wave...)...)
	recompute := f.Plan(st)
	require.Len(t, recompute, 1)
	assert.Equal(t, core.TaskKindCompute, recompute[0].Kind)
	assert.Equal(t, []string{stepCorrection}, recompute[0].DependsOnKinds)
}

func TestTransactionRules_AreWarningsAndSensIsOptional(t *testing.T) {
	f := compFormat(t)
	doc := core.NewDocument()
	item := addCompTx(doc, compTx{montant: "10", date: "2026-03-01", devise: "EUR", typeOperation: "OT10", emetteurLibre: "A", beneficiaireLibre: "B"})
	issues := f.Validate(doc)
	for p, rule := range map[string]string{
		"refEmission": "transaction_origine_requise_sauf_ot07", "paysOrigine": "transaction_origine_requise_sauf_ot07",
		"refReception": "transaction_arrivee_requise_sauf_ot06", "paysArrivee": "transaction_arrivee_requise_sauf_ot06",
	} {
		_, found := findIssue(issues, "rule:"+rule+"@work.transactions[]", core.SeverityWarning, item.Child(p))
		if !found {
			_, found = findIssue(issues, "rule:"+rule, core.SeverityWarning, item.Child(p))
		}
		assert.True(t, found, "%s is a working-node obligation (warning)", p)
	}
	_, found := findIssue(issues, core.CodeRequired, core.SeverityBlocking, item.Child("sens"))
	assert.False(t, found, "sens is optional")
	_, found = findIssue(issues, core.CodeRequired, core.SeverityWarning, item.Child("sens"))
	assert.False(t, found)
}

func TestTools_ReadDocumentReportsTotalAndTruncation(t *testing.T) {
	f := planFormat(t)
	doc := core.NewDocument()
	for i := 0; i < 450; i++ {
		item := doc.AddItem("work.transactions", "transactions")
		doc.Values[item.Child("montant")] = core.Value{Value: "1", Origin: core.OriginManual}
	}
	tool := f.Tools(core.ToolContext{Format: f, Doc: doc})["read_document"]
	assert.Contains(t, tool.Description, "400")
	var out struct {
		Values    []any  `json:"values"`
		Total     int    `json:"total"`
		Truncated bool   `json:"truncated"`
		Type      string `json:"type"`
	}
	require.NoError(t, json.Unmarshal([]byte(tool.Call(context.Background(), json.RawMessage(`{}`))), &out))
	assert.Len(t, out.Values, 400)
	assert.Equal(t, 450, out.Total)
	assert.True(t, out.Truncated)
	assert.Equal(t, core.ClientDataType, out.Type)
}

func TestCompute_IgnoresIbanWithBadChecksum(t *testing.T) {
	f := compFormat(t)
	doc, tx, iban := derivedDoc()
	doc.Values[iban.Child("iban")] = core.Value{Value: "DE89370400440532013001", Origin: core.OriginLLM} // checksum broken
	compApply(t, f, doc, f.Compute(doc))
	_, has := doc.Values[tx.Child("paysOrigine")]
	assert.False(t, has, "an IBAN whose checksum fails gives no country")
}

func TestApplyPlacement_PartialOwnerLossIsReportedAndTransactionsMerge(t *testing.T) {
	f := planFormat(t)
	raw := json.RawMessage(`{"persons":[{"kind":"pp","objects":[{"table":"persons","object_id":"p-1"}]}],
	 "financial_refs":[{"ref_kind":"Iban","owners":[{"person_index":0,"lien":"titulaire"},{"person_index":5,"lien":"co-titulaire"}],"objects":[{"table":"accounts","object_id":"a-1"}]}],
	 "transactions":[
	  {"objects":[{"table":"transactions","object_id":"t-1"}]},
	  {"objects":[{"table":"transactions","object_id":"t-1"},{"table":"accounts","object_id":"a-1"}]}]}`)
	doc := core.NewDocument()
	require.NoError(t, f.ApplyStructured(stepPlacement, doc, raw))
	require.Len(t, doc.Items["work.transactions"], 1, "the same transaction given with and without its account is one")
	tx := core.Path("work.transactions[" + doc.Items["work.transactions"][0] + "]")
	assert.Equal(t, []core.ObjectRef{{Table: "transactions", ObjectID: "t-1"}, {Table: "accounts", ObjectID: "a-1"}}, doc.ItemSources[tx])
	assert.Len(t, doc.Items["personnes.pp[pp-1].referencesFinancieres.referenceFinanciereIban"], 1)

	qs := f.(core.StructuredQuestioner).StructuredQuestions(stepPlacement, raw)
	require.Len(t, qs, 1)
	assert.Contains(t, qs[0].Text, "1 titulaire(s) non déclaré(s)")
	assert.Contains(t, qs[0].Text, "accounts/a-1")
}
