package tracfin_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/usecases/sargen/core"
)

func autoCtx(refs ...core.ObjectRef) core.StructuredContext {
	sc := linkedTo()
	sc.ContextOnly = map[core.ObjectRef]bool{}
	for _, r := range refs {
		sc.ContextOnly[r] = true
	}
	return sc
}

var autoCompany = core.ObjectRef{Table: "companies", ObjectID: "c-1"}

// An auto-added object is context: a person made only of auto-added objects is
// not declared, the analyst is asked, the references it owned are not declared
// either, and the other persons are renumbered.
func TestPlacement_AutoAddedPersonIsNotDeclared(t *testing.T) {
	f := planFormat(t)
	raw := json.RawMessage(`{"persons":[
	  {"kind":"pm","objects":[{"table":"companies","object_id":"c-1"}]},
	  {"kind":"pp","objects":[{"table":"persons","object_id":"p-1"}]}],
	 "financial_refs":[
	  {"ref_kind":"Iban","owners":[{"person_index":1,"lien":"titulaire"}],"objects":[{"table":"accounts","object_id":"a-1"}]},
	  {"ref_kind":"Iban","owners":[{"person_index":0,"lien":"titulaire"}],"objects":[{"table":"accounts","object_id":"a-2"}]}],
	 "transactions":[]}`)
	doc := core.NewDocument()
	sc := autoCtx(autoCompany)
	require.NoError(t, f.ApplyStructured(stepPlacement, doc, raw, sc))

	assert.Empty(t, doc.Items["personnes.pm"], "no declared company from an auto-added object")
	require.Len(t, doc.Items["personnes.pp"], 1)
	person := core.Path("personnes.pp[" + doc.Items["personnes.pp"][0] + "]")
	// The reference of the kept person (index 1 for the model, 0 once the company is out).
	ibans := doc.Items[person.Child("referencesFinancieres").Child("referenceFinanciereIban")]
	require.Len(t, ibans, 1)
	assert.Equal(t, core.ObjectRef{Table: "accounts", ObjectID: "a-1"},
		doc.ItemSources[person.Child("referencesFinancieres").Child("referenceFinanciereIban["+ibans[0]+"]")][0])

	qs := f.(core.StructuredQuestioner).StructuredQuestions(stepPlacement, raw, doc, sc)
	var texts []string
	for _, q := range qs {
		texts = append(texts, q.Text)
	}
	require.Len(t, texts, 2, "%v", texts)
	assert.Contains(t, texts[0], "personne non déclarée : companies/c-1")
	assert.Contains(t, texts[0], "ajouté automatiquement")
	assert.Contains(t, texts[1], "référence financière sans titulaire déclaré : accounts/a-2")
}

// A person that also has an analyst-chosen object is declared, the auto-added
// object stays one of its sources.
func TestPlacement_PersonWithAnAnalystObjectKeepsAutoAddedSource(t *testing.T) {
	f := planFormat(t)
	raw := json.RawMessage(`{"persons":[{"kind":"pm","objects":[{"table":"persons","object_id":"p-1"},{"table":"companies","object_id":"c-1"}]}],
	 "financial_refs":[],"transactions":[]}`)
	doc := core.NewDocument()
	require.NoError(t, f.ApplyStructured(stepPlacement, doc, raw, autoCtx(autoCompany)))
	require.Len(t, doc.Items["personnes.pm"], 1)
	src := doc.ItemSources[core.Path("personnes.pm["+doc.Items["personnes.pm"][0]+"]")]
	assert.Contains(t, src, autoCompany)
}

// Without the marker nothing changes.
func TestPlacement_NoMarkerDeclaresTheCompany(t *testing.T) {
	f := planFormat(t)
	raw := json.RawMessage(`{"persons":[{"kind":"pm","objects":[{"table":"companies","object_id":"c-1"}]}],"financial_refs":[],"transactions":[]}`)
	doc := core.NewDocument()
	require.NoError(t, f.ApplyStructured(stepPlacement, doc, raw, linkedTo()))
	assert.Len(t, doc.Items["personnes.pm"], 1)
}

// ---- fill budget ----

func TestFillLimitsScaleWithInputsAndAreCapped(t *testing.T) {
	f := planFormat(t)
	for _, tc := range []struct{ inputs, turns, calls int }{{1, 16, 32}, {10, 25, 50}, {15, 30, 60}, {50, 30, 60}} {
		doc := planDoc(0, 0)
		for i := 0; i < tc.inputs; i++ {
			k := doc.AddItem("work.transactions", "transactions")
			doc.ItemSources[k] = []core.ObjectRef{{Table: "transactions", ObjectID: string(rune('a'+i%26)) + string(rune('a'+i/26))}}
		}
		// A batch holds at most 10 transactions: use the person task for larger n.
		p := doc.AddItem("personnes.pp", "pp")
		for i := 0; i < tc.inputs; i++ {
			doc.ItemSources[p] = append(doc.ItemSources[p], core.ObjectRef{Table: "persons", ObjectID: string(rune('a'+i%26)) + string(rune('a'+i/26))})
		}
		var person *core.TaskSpec
		for _, s := range f.Plan(core.PlanState{Doc: doc, Done: planDone(core.TaskSpec{Kind: stepUnderstanding}, core.TaskSpec{Kind: stepPlacement})}) {
			if s.Kind == stepFillPerson {
				s := s
				person = &s
			}
		}
		require.NotNil(t, person)
		require.Len(t, person.Inputs, tc.inputs)
		assert.Equal(t, tc.turns, person.Limits.MaxTurns, "inputs=%d", tc.inputs)
		assert.Equal(t, tc.calls, person.Limits.MaxToolCalls, "inputs=%d", tc.inputs)
		assert.Equal(t, min(180+8*tc.inputs, 420), person.Limits.TimeoutSeconds, "inputs=%d", tc.inputs)
	}
}

func TestFillLimitsLargeBatchIsCappedAt30And60(t *testing.T) {
	f := planFormat(t)
	doc := planDoc(0, 0)
	for i := 0; i < 10; i++ {
		k := doc.AddItem("work.transactions", "transactions")
		doc.ItemSources[k] = []core.ObjectRef{{Table: "transactions", ObjectID: string(rune('a' + i))}}
	}
	var tx *core.TaskSpec
	for _, s := range f.Plan(core.PlanState{Doc: doc, Done: planDone(core.TaskSpec{Kind: stepUnderstanding}, core.TaskSpec{Kind: stepPlacement})}) {
		if s.Kind == stepFillTransaction {
			s := s
			tx = &s
		}
	}
	require.NotNil(t, tx)
	assert.Equal(t, 25, tx.Limits.MaxTurns)
	assert.Equal(t, 50, tx.Limits.MaxToolCalls)
}

// An account used as a financial reference does not make a person: a person
// carried only by it (next to a context object) is not declared.
func TestPlacement_PersonCarriedOnlyByAReferenceObjectIsNotDeclared(t *testing.T) {
	f := planFormat(t)
	raw := json.RawMessage(`{"persons":[
	  {"kind":"pp","objects":[{"table":"persons","object_id":"p-1"}]},
	  {"kind":"pm","objects":[{"table":"companies","object_id":"c-1"},{"table":"accounts","object_id":"a-1"}]}],
	 "financial_refs":[{"ref_kind":"Iban","owners":[{"person_index":0,"lien":"titulaire"}],"objects":[{"table":"accounts","object_id":"a-1"}]}],
	 "transactions":[]}`)
	doc := core.NewDocument()
	sc := autoCtx(autoCompany)
	require.NoError(t, f.ApplyStructured(stepPlacement, doc, raw, sc))
	assert.Empty(t, doc.Items["personnes.pm"])
	assert.Len(t, doc.Items["personnes.pp"], 1)
}

// A transaction line made only of a context object is not created.
func TestPlacement_TransactionOfAContextObjectIsNotCreated(t *testing.T) {
	f := planFormat(t)
	raw := json.RawMessage(`{"persons":[{"kind":"pp","objects":[{"table":"persons","object_id":"p-1"}]}],"financial_refs":[],
	 "transactions":[{"object":{"table":"transactions","object_id":"t-1"}},{"object":{"table":"transactions","object_id":"t-ctx"}}]}`)
	doc := core.NewDocument()
	sc := autoCtx(core.ObjectRef{Table: "transactions", ObjectID: "t-ctx"})
	require.NoError(t, f.ApplyStructured(stepPlacement, doc, raw, sc))
	require.Len(t, doc.Items["work.transactions"], 1)
	qs := f.(core.StructuredQuestioner).StructuredQuestions(stepPlacement, raw, doc, sc)
	require.Len(t, qs, 1)
	assert.Contains(t, qs[0].Text, "ligne d'opération non créée : transactions/t-ctx")
}

// A context-only account is a reference of a declared person only when the data
// links it to that person.
func TestPlacement_ContextAccountReferenceNeedsALinkToItsOwner(t *testing.T) {
	f := planFormat(t)
	raw := json.RawMessage(`{"persons":[{"kind":"pp","objects":[{"table":"persons","object_id":"p-1"}]}],
	 "financial_refs":[{"ref_kind":"Iban","owners":[{"person_index":0,"lien":"titulaire"}],"objects":[{"table":"accounts","object_id":"a-ctx"}]}],
	 "transactions":[]}`)
	account := core.ObjectRef{Table: "accounts", ObjectID: "a-ctx"}
	person := core.ObjectRef{Table: "persons", ObjectID: "p-1"}

	unlinked := core.StructuredContext{ContextOnly: map[core.ObjectRef]bool{account: true},
		LinkedObjects: func(core.ObjectRef) []core.ObjectRef { return nil }}
	doc := core.NewDocument()
	require.NoError(t, f.ApplyStructured(stepPlacement, doc, raw, unlinked))
	assert.Empty(t, doc.Items["personnes.pp[pp-1].referencesFinancieres.referenceFinanciereIban"])
	qs := f.(core.StructuredQuestioner).StructuredQuestions(stepPlacement, raw, doc, unlinked)
	require.Len(t, qs, 1)
	assert.Contains(t, qs[0].Text, "référence financière non déclarée : accounts/a-ctx")

	linked := core.StructuredContext{ContextOnly: map[core.ObjectRef]bool{account: true},
		LinkedObjects: func(ref core.ObjectRef) []core.ObjectRef {
			if ref == account {
				return []core.ObjectRef{person}
			}
			return nil
		}}
	doc = core.NewDocument()
	require.NoError(t, f.ApplyStructured(stepPlacement, doc, raw, linked))
	assert.Len(t, doc.Items["personnes.pp[pp-1].referencesFinancieres.referenceFinanciereIban"], 1)
}

// A context-only person next to an analyst transaction is not declared: the
// analyst object must come from a person-, company- or partner-like table.
func TestPlacement_AnalystTransactionDoesNotCarryAPerson(t *testing.T) {
	f := planFormat(t)
	raw := json.RawMessage(`{"persons":[{"kind":"pp","objects":[{"table":"persons","object_id":"p-ctx"},{"table":"transactions","object_id":"t-1"}]}],
	 "financial_refs":[],"transactions":[]}`)
	person := core.ObjectRef{Table: "persons", ObjectID: "p-ctx"}
	txn := core.ObjectRef{Table: "transactions", ObjectID: "t-1"}
	sc := core.StructuredContext{
		ContextOnly: map[core.ObjectRef]bool{person: true},
		Semantics: map[core.ObjectRef]models.SemanticType{
			person: models.SemanticTypePerson, txn: models.SemanticTypeTransaction,
		},
	}
	doc := core.NewDocument()
	require.NoError(t, f.ApplyStructured(stepPlacement, doc, raw, sc))
	assert.Empty(t, doc.Items["personnes.pp"])

	// With an analyst-chosen person-like object it is declared.
	sc.ContextOnly = nil
	doc = core.NewDocument()
	require.NoError(t, f.ApplyStructured(stepPlacement, doc, raw, sc))
	assert.Len(t, doc.Items["personnes.pp"], 1)
}

// A person made only of analyst-chosen objects is declared whatever the table
// semantic (account, event, other, unset): the check only guards persons that
// also hold context objects.
func TestPlacement_AnalystOnlyPersonIgnoresTheTableSemantic(t *testing.T) {
	f := planFormat(t)
	for _, sem := range []models.SemanticType{models.SemanticTypeAccount, models.SemanticTypeEvent,
		models.SemanticTypeOther, models.SemanticTypeUnset, models.SemanticTypeTransaction} {
		raw := json.RawMessage(`{"persons":[{"kind":"pp","objects":[{"table":"customers","object_id":"c-9"}]}],"financial_refs":[],"transactions":[]}`)
		obj := core.ObjectRef{Table: "customers", ObjectID: "c-9"}
		sc := core.StructuredContext{Semantics: map[core.ObjectRef]models.SemanticType{obj: sem}}
		doc := core.NewDocument()
		require.NoError(t, f.ApplyStructured(stepPlacement, doc, raw, sc))
		assert.Len(t, doc.Items["personnes.pp"], 1, "semantic %q", sem)
	}
}
