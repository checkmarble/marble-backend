package tracfin_test

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/usecases/sargen/core"
	"github.com/checkmarble/marble-backend/usecases/sargen/formats/tracfin"
)

// A person made only of auto-added objects is kept, and the analyst asked, when
// it is directly involved in a declared operation.

func obj(table, id string) core.ObjectRef { return core.ObjectRef{Table: table, ObjectID: id} }

// caseACtx is case A: the analyst chose the transactions t-1 and t-2; the account
// a-1 was added as their parent and the holder p-A as the account's parent.
func caseACtx() core.StructuredContext {
	tx1, tx2, acc, holder := obj("transactions", "t-1"), obj("transactions", "t-2"), obj("accounts", "a-1"), obj("persons", "p-A")
	return core.StructuredContext{
		Selection:   []core.ObjectRef{tx1, tx2, acc, holder},
		ContextOnly: map[core.ObjectRef]bool{acc: true, holder: true},
		ReachedFrom: map[core.ObjectRef]core.ObjectRef{acc: tx1, holder: acc},
		LinkedObjects: func(ref core.ObjectRef) []core.ObjectRef {
			switch ref {
			case tx1, tx2:
				return []core.ObjectRef{acc}
			case acc:
				return []core.ObjectRef{holder}
			case holder:
				return []core.ObjectRef{acc}
			}
			return nil
		},
	}
}

const caseAPlacement = `{"persons":[{"kind":"pp","objects":[{"table":"persons","object_id":"p-A"}]}],
 "financial_refs":[{"ref_kind":"Iban","owners":[{"person_index":0,"lien":"titulaire"}],"objects":[{"table":"accounts","object_id":"a-1"}]}],
 "transactions":[{"object":{"table":"transactions","object_id":"t-1"}},{"object":{"table":"transactions","object_id":"t-2"}}]}`

func placeAndAsk(t *testing.T, raw string, sc core.StructuredContext) (*core.Document, []core.QuestionDraft) {
	t.Helper()
	f := planFormat(t)
	doc := core.NewDocument()
	require.NoError(t, f.ApplyStructured(stepPlacement, doc, json.RawMessage(raw), sc))
	return doc, f.(core.StructuredQuestioner).StructuredQuestions(stepPlacement, json.RawMessage(raw), doc, sc)
}

func TestContextPerson_HolderOfAnAutoAccountOfDeclaredTransactionsIsKeptAndAsked(t *testing.T) {
	doc, qs := placeAndAsk(t, caseAPlacement, caseACtx())

	require.Len(t, doc.Items["personnes.pp"], 1, "the holder is declared though context only")
	person := core.Path("personnes.pp[" + doc.Items["personnes.pp"][0] + "]")
	assert.Equal(t, []core.ObjectRef{obj("persons", "p-A")}, doc.ItemSources[person])
	// Its financial reference is kept under the context-reference rule.
	ibans := doc.Items[person.Child("referencesFinancieres").Child("referenceFinanciereIban")]
	require.Len(t, ibans, 1)
	assert.Len(t, doc.Items["work.transactions"], 2)

	require.Len(t, qs, 1)
	q := qs[0]
	assert.True(t, q.Blocking, "export refused until answered")
	assert.Equal(t, []core.Path{person}, q.Paths)
	assert.Contains(t, q.Text, "persons/p-A a été ajouté automatiquement comme titulaire ou dirigeant lié aux opérations déclarées")
	assert.Contains(t, q.Text, "TRACFIN attend que toute personne directement impliquée soit déclarée.")
	require.Len(t, q.Options, 2)
	assert.Equal(t, "Déclarer persons/p-A comme personne impliquée", q.Options[0].Label)
	assert.Empty(t, q.Options[0].Fills)
	assert.Empty(t, q.Options[0].RemoveItem, "accepting writes nothing")
	assert.Equal(t, "Ne pas déclarer", q.Options[1].Label)
	assert.Equal(t, person, q.Options[1].RemoveItem)
}

// Case D: the director of a declared company.
func TestContextPerson_DirectorOfADeclaredCompanyIsKeptAndAsked(t *testing.T) {
	company, director := obj("companies", "c-1"), obj("persons", "d-1")
	sc := core.StructuredContext{
		Selection:   []core.ObjectRef{company, director},
		ContextOnly: map[core.ObjectRef]bool{director: true},
		ReachedFrom: map[core.ObjectRef]core.ObjectRef{director: company},
	}
	raw := `{"persons":[{"kind":"pm","objects":[{"table":"companies","object_id":"c-1"}]},
	  {"kind":"pp","objects":[{"table":"persons","object_id":"d-1"}]}],"financial_refs":[],"transactions":[]}`
	doc, qs := placeAndAsk(t, raw, sc)

	assert.Len(t, doc.Items["personnes.pm"], 1)
	require.Len(t, doc.Items["personnes.pp"], 1, "the director is declared")
	require.Len(t, qs, 1)
	assert.True(t, qs[0].Blocking)
	assert.Equal(t, core.Path("personnes.pp["+doc.Items["personnes.pp"][0]+"]"), qs[0].Options[1].RemoveItem)
}

// A context person that no declared item leads to keeps today's plain question.
func TestContextPerson_NotLinkedToAnyDeclaredItemKeepsThePlainQuestion(t *testing.T) {
	other, holder := obj("transactions", "t-9"), obj("persons", "p-Z")
	sc := core.StructuredContext{
		Selection:   []core.ObjectRef{obj("transactions", "t-1"), other, holder},
		ContextOnly: map[core.ObjectRef]bool{holder: true},
		// Reached from an analyst object that the output does not declare.
		ReachedFrom: map[core.ObjectRef]core.ObjectRef{holder: other},
	}
	raw := `{"persons":[{"kind":"pp","objects":[{"table":"persons","object_id":"p-Z"}]}],"financial_refs":[],
	 "transactions":[{"object":{"table":"transactions","object_id":"t-1"}}]}`
	doc, qs := placeAndAsk(t, raw, sc)

	assert.Empty(t, doc.Items["personnes.pp"])
	require.Len(t, qs, 1)
	assert.Contains(t, qs[0].Text, "personne non déclarée : persons/p-Z")
	assert.False(t, qs[0].Blocking)
	assert.Empty(t, qs[0].Options)

	// No reached-from information at all: same.
	sc.ReachedFrom = nil
	doc, qs = placeAndAsk(t, raw, sc)
	assert.Empty(t, doc.Items["personnes.pp"])
	require.Len(t, qs, 1)
	assert.Contains(t, qs[0].Text, "personne non déclarée")
}

// An object that cannot carry an entity (an account, by its semantic) is not
// kept as a person, whatever leads to it.
func TestContextPerson_ObjectThatCannotCarryAnEntityIsNotKept(t *testing.T) {
	sc := caseACtx()
	sc.Semantics = map[core.ObjectRef]models.SemanticType{obj("persons", "p-A"): models.SemanticTypeAccount}
	doc, qs := placeAndAsk(t, caseAPlacement, sc)

	assert.Empty(t, doc.Items["personnes.pp"])
	for _, q := range qs {
		assert.False(t, q.Blocking)
	}
}

func TestContextPerson_AtMostFivePersonsAreKeptTheOthersKeepThePlainQuestion(t *testing.T) {
	tx := obj("transactions", "t-1")
	sc := core.StructuredContext{
		Selection: []core.ObjectRef{tx}, ContextOnly: map[core.ObjectRef]bool{},
		ReachedFrom: map[core.ObjectRef]core.ObjectRef{},
	}
	var persons []string
	for i := 1; i <= 7; i++ {
		o := obj("persons", fmt.Sprintf("p-%d", i))
		sc.Selection = append(sc.Selection, o)
		sc.ContextOnly[o] = true
		sc.ReachedFrom[o] = tx
		persons = append(persons, fmt.Sprintf(`{"kind":"pp","objects":[{"table":"persons","object_id":"p-%d"}]}`, i))
	}
	raw := `{"persons":[` + strings.Join(persons, ",") + `],"financial_refs":[],
	 "transactions":[{"object":{"table":"transactions","object_id":"t-1"}}]}`
	doc, qs := placeAndAsk(t, raw, sc)

	assert.Len(t, doc.Items["personnes.pp"], 5)
	blocking, plain := 0, 0
	for _, q := range qs {
		if q.Blocking {
			blocking++
		} else if strings.Contains(q.Text, "personne non déclarée") {
			plain++
		}
	}
	assert.Equal(t, 5, blocking)
	assert.Equal(t, 2, plain)
	// The first five of the output, in order.
	for i, k := range doc.Items["personnes.pp"] {
		assert.Equal(t, []core.ObjectRef{obj("persons", fmt.Sprintf("p-%d", i+1))}, doc.ItemSources[core.Path("personnes.pp["+k+"]")])
	}
}

// Declining: the transactions switch to free text (name and IBAN), the person
// goes, the operation lines are recomputed and nothing dangles.
func TestContextPerson_DecliningRewritesTheTransactionsAndRemovesThePerson(t *testing.T) {
	f := planFormat(t)
	doc, _ := placeAndAsk(t, caseAPlacement, caseACtx())
	person := core.Path("personnes.pp[" + doc.Items["personnes.pp"][0] + "]")
	iban := person.Child("referencesFinancieres").Child("referenceFinanciereIban[" +
		doc.Items[person.Child("referencesFinancieres").Child("referenceFinanciereIban")][0] + "]")
	// What the fill tasks write afterwards.
	set := func(p core.Path, v string) { doc.Values[p] = core.Value{Value: v, Origin: core.OriginLLM} }
	set(person.Child("etatCivil").Child("nomNaissance"), "Zorg")
	prenom := doc.AddItem(person.Child("etatCivil").Child("prenom"), "prenom")
	set(prenom, "Jean")
	set(iban.Child("iban"), "FR1420041010050500013M02606")
	for _, k := range doc.Items["work.transactions"] {
		tx := core.Path("work.transactions[" + k + "]")
		set(tx.Child("montant"), "14000")
		set(tx.Child("date"), "2026-03-01")
		set(tx.Child("devise"), "EUR")
		set(tx.Child("typeOperation"), "OT10")
		set(tx.Child("emetteurPersonne"), string(person))
		set(tx.Child("refEmission"), string(iban))
		set(tx.Child("beneficiaireLibre"), "Fournisseur PT")
		set(tx.Child("paysArrivee"), "PT")
	}
	// A link elsewhere in the document that points at the person.
	pm := doc.AddItem("personnes.pm", "pm")
	dirigeant := doc.AddItem(pm.Child("dirigeant"), "dirigeant")
	set(dirigeant.Child("idPersonne"), string(person))
	compApply(t, f, doc, f.Compute(doc))
	require.Len(t, compLines(doc), 1, "one operation line before")

	rw, ok := f.(core.ItemRemovalRewriter)
	require.True(t, ok)
	fills := rw.RewriteBeforeRemoval(doc, person)
	for i := range fills {
		fills[i].Origin = core.OriginAnalyst
	}
	res := core.ApplyFills(f.Spec(), doc, fills, core.ApplyOptions{Overwrite: core.OverwriteAll})
	require.Empty(t, res.Rejected)
	doc.RemoveItem(person)
	compApply(t, f, doc, f.Compute(doc))

	for _, k := range doc.Items["work.transactions"] {
		tx := core.Path("work.transactions[" + k + "]")
		assert.Empty(t, doc.Values[tx.Child("emetteurPersonne")].Value)
		assert.Equal(t, "Jean Zorg", doc.Values[tx.Child("emetteurLibre")].Value)
		assert.Equal(t, "FR1420041010050500013M02606", doc.Values[tx.Child("refEmission")].Value, "the IBAN as text")
		assert.Equal(t, "Fournisseur PT", doc.Values[tx.Child("beneficiaireLibre")].Value)
	}
	assert.Empty(t, doc.Values[dirigeant.Child("idPersonne")].Value, "any other reference is cleared")
	assert.Empty(t, doc.Items["personnes.pp"])
	line := compLines(doc)
	require.Len(t, line, 1, "the operation lines are recomputed")
	assert.Equal(t, "Jean Zorg", compLineValue(doc, line[0], "emetteurTypeEmetteur"))
	assert.Equal(t, "FR1420041010050500013M02606", compLineValue(doc, line[0], "refFinanciereEmission"))
	assert.Equal(t, "FR", compLineValue(doc, line[0], "paysOrigineFonds"), "the origin country follows the IBAN as text")
	for _, is := range tracfin.Validate(f.Spec(), doc) {
		assert.NotContains(t, []string{"dangling_ref", "bad_ref"}, is.Code, "%s: %s", is.Path, is.Message)
	}
}

// A company: its name is the raison sociale; a person without any name falls back
// on its source objects.
func TestContextPerson_FreeTextNameOfACompanyAndOfAnUnfilledPerson(t *testing.T) {
	f := planFormat(t)
	rw := f.(core.ItemRemovalRewriter)
	doc := core.NewDocument()
	pm := doc.AddItem("personnes.pm", "pm")
	doc.Values[pm.Child("activiteCoordonnee").Child("denominationSociale")] = core.Value{Value: "Acme SAS", Origin: core.OriginLLM}
	pp := doc.AddItem("personnes.pp", "pp")
	doc.ItemSources[pp] = []core.ObjectRef{obj("persons", "p-A")}
	tx := doc.AddItem("work.transactions", "transactions")
	doc.Values[tx.Child("emetteurPersonne")] = core.Value{Value: string(pm), Origin: core.OriginLLM}
	doc.Values[tx.Child("beneficiairePersonne")] = core.Value{Value: string(pp), Origin: core.OriginLLM}

	fillsPM := rw.RewriteBeforeRemoval(doc, pm)
	assert.Contains(t, fillsPM, core.Fill{Path: tx.Child("emetteurLibre"), Value: "Acme SAS"})
	fillsPP := rw.RewriteBeforeRemoval(doc, pp)
	assert.Contains(t, fillsPP, core.Fill{Path: tx.Child("beneficiaireLibre"), Value: "persons/p-A"})
	assert.Contains(t, fillsPP, core.Fill{Path: tx.Child("beneficiairePersonne"), Value: ""})
}
