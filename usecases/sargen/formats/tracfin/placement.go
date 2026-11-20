package tracfin

import (
	"bytes"
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/cockroachdb/errors"
	"github.com/invopop/jsonschema"

	"github.com/checkmarble/marble-backend/usecases/sargen/core"
)

const referenceContainerPrefix = "referenceFinanciere"

// liens maps the placement vocabulary to the XSD t_lien codes.
var liens = map[string]string{
	"titulaire":    "Titulaire",
	"co-titulaire": "Co-titulaire",
	"mandataire":   "Mandataire",
}

type placementOutput struct {
	Persons []struct {
		Kind    string           `json:"kind"`
		Objects []core.ObjectRef `json:"objects"`
	} `json:"persons"`
	FinancialRefs []struct {
		RefKind string `json:"ref_kind"`
		Owners  []struct {
			PersonIndex *int   `json:"person_index"`
			Lien        string `json:"lien"`
		} `json:"owners"`
		Objects []core.ObjectRef `json:"objects"`
	} `json:"financial_refs"`
	Transactions []struct {
		// Object is the transaction object. The backend adds the selected objects
		// linked to it (its accounts) as sources.
		Object core.ObjectRef `json:"object"`
	} `json:"transactions"`
}

// referenceKinds lists the financial reference types of the XSD
// (referencesFinancieres children, without their common prefix), sorted.
func (f *format) referenceKinds() []string {
	node := f.spec.NodeAt("personnes.pp[].referencesFinancieres")
	var out []string
	if node == nil {
		return out
	}
	for _, c := range node.Children {
		if k, ok := strings.CutPrefix(c.Name, referenceContainerPrefix); ok && k != "" {
			out = append(out, k)
		}
	}
	slices.Sort(out)
	return out
}

func placementSchema(refKinds []string) jsonschema.Schema {
	enum := make([]string, len(refKinds))
	for i, k := range refKinds {
		enum[i] = `"` + k + `"`
	}
	const obj = `{"type":"object","additionalProperties":false,"required":["table","object_id"],"properties":{"table":{"type":"string"},"object_id":{"type":"string"}}}`
	raw := `{"type":"object","additionalProperties":false,"required":["persons","financial_refs","transactions"],"properties":{
 "persons":{"type":"array","items":{"type":"object","additionalProperties":false,"required":["kind","objects"],"properties":{
   "kind":{"type":"string","enum":["pp","pm","pnc"],"description":"pp = natural person (personne physique), pm = legal entity (personne morale), pnc = uncharacterized person (personne non caractérisée)"},
   "objects":{"type":"array","items":` + obj + `}}}},
 "financial_refs":{"type":"array","items":{"type":"object","additionalProperties":false,"required":["ref_kind","owners","objects"],"properties":{
   "ref_kind":{"type":"string","enum":[` + strings.Join(enum, ",") + `]},
   "owners":{"type":"array","items":{"type":"object","additionalProperties":false,"required":["person_index","lien"],"properties":{
     "person_index":{"type":"integer","description":"position of the person in persons (starting at 0)"},
     "lien":{"type":"string","enum":["titulaire","co-titulaire","mandataire"]}}}},
   "objects":{"type":"array","items":` + obj + `}}}},
 "transactions":{"type":"array","items":{"type":"object","additionalProperties":false,"required":["object"],"properties":{"object":{"description":"the transaction object (never a person object nor a declared account); linked accounts are added by the system","type":"object","additionalProperties":false,"required":["table","object_id"],"properties":{"table":{"type":"string"},"object_id":{"type":"string"}}}}}}}}`
	var s jsonschema.Schema
	if err := json.Unmarshal([]byte(raw), &s); err != nil {
		panic("placement schema: " + err.Error()) // constant above: a programming error
	}
	return s
}

// placementLeftOut is what a placement output proposed and the context rule
// refused: persons without any object of the analyst, and transaction lines
// made only of context objects.
type placementLeftOut struct {
	persons      [][]core.ObjectRef
	transactions []core.ObjectRef
	// contextPersons are the persons made only of context-only objects that the
	// declaration keeps anyway because they are directly involved in a declared
	// operation (see directlyInvolved). They are in the output persons like any
	// declared person; the analyst is asked to confirm each of them.
	contextPersons []contextPerson
}

// contextPerson is a kept person that rests only on objects the server added on
// its own.
type contextPerson struct {
	kind    string
	objects []core.ObjectRef
}

// maxContextPersons caps the persons kept for their direct involvement in a
// declared operation: past it, the others keep the plain question.
const maxContextPersons = 5

// parsePlacement decodes and checks a placement output. References without a
// valid declared owner are returned apart (dropped, reported as a question); any
// other error refuses the whole output.
//
// Objects the server added on its own (core.StructuredContext.ContextOnly) are
// context to read and cite, never declared on their own:
//   - a person is declared only if at least one of its objects is chosen by the
//     analyst (not context only). When the person also holds context-only
//     objects, that analyst object must in addition not be the object of a
//     financial reference of the output and must come from a person-, company-
//     or partner-like table (a transaction or an account does not make a
//     person; an unknown table semantic is not held against it). A person made
//     only of analyst objects is declared whatever their table, even when one
//     of them is also a reference object. The others are returned in left, the
//     references they owned lose them as an owner and the remaining persons are
//     renumbered;
//   - a person made only of context-only objects is still kept, at most
//     maxContextPersons per output (in output order), when it is directly
//     involved in a declared operation: one of its objects was added through a
//     chain of auto-added rows that starts at an object chosen by the analyst
//     and carried by a declared item (a declared person, a transaction or a
//     financial reference of the output). Such a person is the holder of the
//     account of a declared transaction, or the director of a declared
//     company, and TRACFIN expects it declared. The analyst is asked to confirm
//     it (StructuredQuestions);
//   - a transaction made only of a context object is not created (left);
//   - a financial reference made only of context objects is kept only when the
//     data links one of them to an object of one of its owners; otherwise it is
//     dropped and the analyst is asked.
func (f *format) parsePlacement(raw json.RawMessage, sc core.StructuredContext) (out placementOutput, dropped []droppedRef, left placementLeftOut, err error) {
	if err := json.NewDecoder(bytes.NewReader(raw)).Decode(&out); err != nil {
		return out, nil, left, errors.Wrap(err, "unreadable placement output")
	}
	refKinds := f.referenceKinds()
	for i, p := range out.Persons {
		if !slices.Contains(personKinds, p.Kind) {
			return out, nil, left, errors.Newf("person %d: unknown kind %q (pp, pm or pnc)", i, p.Kind)
		}
		if err := checkObjects(p.Objects); err != nil {
			return out, nil, left, errors.Wrapf(err, "person %d", i)
		}
	}
	for i, r := range out.FinancialRefs {
		if !slices.Contains(refKinds, r.RefKind) {
			return out, nil, left, errors.Newf("financial reference %d: unknown ref_kind %q (%s)", i, r.RefKind, strings.Join(refKinds, ", "))
		}
		if err := checkObjects(r.Objects); err != nil {
			return out, nil, left, errors.Wrapf(err, "financial reference %d", i)
		}
	}
	for i, t := range out.Transactions {
		if err := checkObjects([]core.ObjectRef{t.Object}); err != nil {
			return out, nil, left, errors.Wrapf(err, "transaction %d", i)
		}
	}

	usedAsReference := map[core.ObjectRef]bool{}
	for _, r := range out.FinancialRefs {
		for _, o := range r.Objects {
			usedAsReference[o] = true
		}
	}
	// newIndex maps the index the model used to the index among the kept persons
	// (-1: left out).
	asked := len(out.Persons)
	newIndex := make([]int, asked)
	keptPersons := out.Persons[:0:0]
	carriedPerson := make([]bool, asked)
	for i, p := range out.Persons {
		// The reference-object and table-semantic restrictions only apply to a
		// person that mixes in context-only objects (the case they guard
		// against): a person made only of analyst-chosen objects keeps the plain
		// behaviour, whatever the table and even on a reference object.
		mixed := slices.ContainsFunc(p.Objects, sc.IsContextOnly)
		carried := slices.ContainsFunc(p.Objects, func(o core.ObjectRef) bool {
			return !sc.IsContextOnly(o) && (!mixed || (!usedAsReference[o] && sc.CanCarryEntity(o)))
		})
		carriedPerson[i] = carried
	}
	declared := declaredObjects(out, carriedPerson, sc)
	for i, p := range out.Persons {
		if !carriedPerson[i] {
			if len(left.contextPersons) < maxContextPersons && directlyInvolved(p.Objects, usedAsReference, declared, sc) {
				left.contextPersons = append(left.contextPersons, contextPerson{kind: p.Kind, objects: slices.Clone(p.Objects)})
			} else {
				newIndex[i] = -1
				left.persons = append(left.persons, slices.Clone(p.Objects))
				continue
			}
		}
		newIndex[i] = len(keptPersons)
		keptPersons = append(keptPersons, p)
	}
	out.Persons = keptPersons

	kept := out.FinancialRefs[:0:0]
	for _, r := range out.FinancialRefs {
		owners := r.Owners[:0:0]
		var seen []int
		lost := 0
		for _, o := range r.Owners {
			if o.PersonIndex == nil || *o.PersonIndex < 0 || *o.PersonIndex >= asked || newIndex[*o.PersonIndex] < 0 {
				lost++
				continue
			}
			if _, ok := liens[o.Lien]; !ok {
				lost++
				continue
			}
			idx := newIndex[*o.PersonIndex]
			if slices.Contains(seen, idx) {
				continue
			}
			seen = append(seen, idx)
			o.PersonIndex = &idx
			owners = append(owners, o)
		}
		if len(owners) > 0 && !f.referenceAttached(r.Objects, owners, out.Persons, sc) {
			dropped = append(dropped, droppedRef{objects: r.Objects, wholly: true, unlinked: true})
			continue
		}
		if len(owners) == 0 || lost > 0 {
			dropped = append(dropped, droppedRef{objects: r.Objects, wholly: len(owners) == 0, lostOwners: lost})
		}
		if len(owners) == 0 {
			continue
		}
		r.Owners = owners
		kept = append(kept, r)
	}
	out.FinancialRefs = kept

	txs := out.Transactions[:0:0]
	for _, t := range out.Transactions {
		if sc.IsContextOnly(t.Object) {
			left.transactions = append(left.transactions, t.Object)
			continue
		}
		txs = append(txs, t)
	}
	out.Transactions = txs
	return out, dropped, left, nil
}

// declaredObjects returns the objects carried by the items the output declares
// on the analyst's choice: the objects of the persons that rest on an analyst
// object, the transaction objects that are not context only, and the objects of
// the financial references that are not context only.
func declaredObjects(out placementOutput, carriedPerson []bool, sc core.StructuredContext) map[core.ObjectRef]bool {
	declared := map[core.ObjectRef]bool{}
	for i, p := range out.Persons {
		if carriedPerson[i] {
			for _, o := range p.Objects {
				declared[o] = true
			}
		}
	}
	for _, t := range out.Transactions {
		if !sc.IsContextOnly(t.Object) {
			declared[t.Object] = true
		}
	}
	for _, r := range out.FinancialRefs {
		for _, o := range r.Objects {
			if !sc.IsContextOnly(o) {
				declared[o] = true
			}
		}
	}
	return declared
}

// directlyInvolved tells whether a person made only of context-only objects is
// directly involved in a declared operation: one of its objects, that can carry
// an entity (person-, company- or partner-like table, or unknown; not an object
// the output uses as a financial reference), was reached through auto-added rows
// from an object chosen by the analyst that a declared item carries.
func directlyInvolved(objs []core.ObjectRef, usedAsReference, declared map[core.ObjectRef]bool, sc core.StructuredContext) bool {
	return slices.ContainsFunc(objs, func(o core.ObjectRef) bool {
		if usedAsReference[o] || !sc.CanCarryEntity(o) {
			return false
		}
		choice, ok := sc.ChoiceBehind(o)
		return ok && declared[choice]
	})
}

// referenceAttached tells whether a financial reference may be declared: one of
// its objects is not context only, or the data links one of its (context)
// objects to an object of one of its owners.
func (f *format) referenceAttached(objs []core.ObjectRef, owners []struct {
	PersonIndex *int   `json:"person_index"`
	Lien        string `json:"lien"`
}, persons []struct {
	Kind    string           `json:"kind"`
	Objects []core.ObjectRef `json:"objects"`
}, sc core.StructuredContext,
) bool {
	if slices.ContainsFunc(objs, func(o core.ObjectRef) bool { return !sc.IsContextOnly(o) }) {
		return true
	}
	for _, ow := range owners {
		for _, po := range persons[*ow.PersonIndex].Objects {
			for _, o := range objs {
				if slices.Contains(sc.Linked(o), po) || slices.Contains(sc.Linked(po), o) {
					return true
				}
			}
		}
	}
	return false
}

// droppedRef is a financial reference left out (wholly: no valid owner, or
// unlinked: a context-only object the data does not link to its owners) or
// declared with fewer owners than listed.
type droppedRef struct {
	objects    []core.ObjectRef
	wholly     bool
	unlinked   bool
	lostOwners int
}

// StructuredQuestions reports what a placement output left out: the persons
// made only of auto-added objects and the financial references without a valid
// declared owner (core.StructuredQuestioner).
func (f *format) StructuredQuestions(kind string, raw json.RawMessage, applied *core.Document, sc core.StructuredContext) []core.QuestionDraft {
	if kind != StepPlacement {
		return nil
	}
	_, dropped, left, err := f.parsePlacement(raw, sc)
	if err != nil {
		return nil
	}
	var out []core.QuestionDraft
	names := func(objs []core.ObjectRef) string {
		n := make([]string, len(objs))
		for i, o := range objs {
			n[i] = o.Table + "/" + o.ObjectID
		}
		return strings.Join(n, ", ")
	}
	for _, objs := range left.persons {
		out = append(out, core.QuestionDraft{Text: "personne non déclarée : " + names(objs) +
			" n'est portée par aucun objet choisi par l'analyste (objet de contexte ajouté automatiquement, ou déjà utilisé comme référence financière) ; à déclarer manuellement si nécessaire"})
	}
	for _, o := range left.transactions {
		out = append(out, core.QuestionDraft{Text: "ligne d'opération non créée : " + names([]core.ObjectRef{o}) +
			" est un objet de contexte ajouté automatiquement ; à ajouter manuellement si nécessaire"})
	}
	for _, d := range dropped {
		list := names(d.objects)
		if d.unlinked {
			out = append(out, core.QuestionDraft{Text: "référence financière non déclarée : " + list +
				" est un objet de contexte que les données ne rattachent à aucun de ses titulaires ; à déclarer manuellement si nécessaire"})
		} else if d.wholly {
			out = append(out, core.QuestionDraft{Text: "référence financière sans titulaire déclaré : " + list})
		} else {
			out = append(out, core.QuestionDraft{Text: fmt.Sprintf("référence financière : %d titulaire(s) non déclaré(s) ignoré(s) : %s", d.lostOwners, list)})
		}
	}
	out = append(out, f.contextPersonQuestions(left.contextPersons, applied, names)...)
	out = append(out, f.mixedUseQuestions(raw, applied)...)
	return out
}

// contextPersonQuestions asks the analyst to confirm each person kept only for
// its direct involvement. The question is blocking (the export is refused until
// it is answered) and has two options: declare the person (nothing is written,
// the question is answered) or do not declare it, which replaces the references
// to the person in the transactions by free text and removes the person (see
// RewriteBeforeRemoval).
func (f *format) contextPersonQuestions(persons []contextPerson, applied *core.Document, names func([]core.ObjectRef) string) []core.QuestionDraft {
	if applied == nil {
		return nil
	}
	var out []core.QuestionDraft
	for _, p := range persons {
		container := core.Path("personnes." + p.kind)
		item := core.Path("")
		for _, k := range applied.Items[container] {
			cand := core.Path(string(container) + "[" + k + "]")
			if sameObjects(applied.ItemSources[cand], p.objects) {
				item = cand
				break
			}
		}
		if item == "" {
			continue
		}
		who := names(p.objects)
		out = append(out, core.QuestionDraft{
			Paths:    []core.Path{item},
			Blocking: true,
			Text: who + " a été ajouté automatiquement comme titulaire ou dirigeant lié aux opérations déclarées. " +
				"TRACFIN attend que toute personne directement impliquée soit déclarée.",
			Options: []core.QuestionOption{
				{Label: "Déclarer " + who + " comme personne impliquée"},
				{Label: "Ne pas déclarer", RemoveItem: item},
			},
		})
	}
	return out
}

// mixedUseQuestions asks the analyst to check the transactions whose object is
// also the object of a declared person or financial reference (a model where
// the transaction row carries its counterparty or IBAN is legitimate, but so is
// a model confusion). The path is the working transaction of the applied
// document whose first source is that object (the newest one).
func (f *format) mixedUseQuestions(raw json.RawMessage, applied *core.Document) []core.QuestionDraft {
	var in placementOutput
	if err := json.Unmarshal(raw, &in); err != nil {
		return nil
	}
	declared := map[core.ObjectRef]bool{}
	for _, p := range in.Persons {
		for _, o := range p.Objects {
			declared[o] = true
		}
	}
	for _, r := range in.FinancialRefs {
		for _, o := range r.Objects {
			declared[o] = true
		}
	}
	var out []core.QuestionDraft
	var seen []core.ObjectRef
	for _, t := range in.Transactions {
		if slices.Contains(seen, t.Object) {
			continue
		}
		seen = append(seen, t.Object)
		if !declared[t.Object] || applied == nil {
			continue
		}
		path := core.Path("")
		for _, k := range applied.Items[transactionsContainer] {
			item := core.Path(string(transactionsContainer) + "[" + k + "]")
			if src := applied.ItemSources[item]; len(src) > 0 && src[0] == t.Object {
				path = item
			}
		}
		if path == "" {
			continue
		}
		out = append(out, core.QuestionDraft{
			Paths: []core.Path{path},
			Text: fmt.Sprintf("L'objet %s/%s est utilisé à la fois comme transaction et comme personne ou référence financière : à vérifier.",
				t.Object.Table, t.Object.ObjectID),
		})
	}
	return out
}

// sameObjects tells whether two object lists hold the same objects, in any
// order.
func sameObjects(a, b []core.ObjectRef) bool {
	return len(a) == len(b) && !slices.ContainsFunc(a, func(o core.ObjectRef) bool { return !slices.Contains(b, o) })
}

// ApplyStructured applies the output of the placement step (the understanding
// note is only checked, see cleanUnderstanding): it creates the
// person items (with their source objects), the financial references under each
// declared owner (a joint account is created under each of them, with its own
// lien, the elements sharing the same sources) and the working transactions.
//
// Dedupe: persons with the same type and the same objects are one person;
// references of the same type and objects are one reference (their owners are
// merged); transactions are identified by their transaction object (one id per
// entry). The object of a transaction may also be the object of a declared
// person or reference: it is accepted and the analyst is asked to check it
// (see mixedUseQuestions). An object shared by two distinct persons stays
// allowed.
//
// A transaction's sources are its objects plus the selected objects linked to
// its transaction object by the data model that are the objects of a declared
// financial reference (its account): the model does not have to list them.
//
// A reference without a valid declared owner is dropped; any other invalid
// index, reference type, lien or object is an error and the caller keeps its
// copy of the document untouched.
func (f *format) ApplyStructured(kind string, doc *core.Document, raw json.RawMessage, sc core.StructuredContext) error {
	switch kind {
	case StepPlacement:
	case StepUnderstanding:
		// The note changes nothing in the document: it is only checked (the
		// engine keeps its valid items, see CleanStructured).
		_, _, err := cleanUnderstanding(raw, sc)
		return err
	default:
		return errors.Newf("unknown structured output: %q", kind)
	}
	out, _, _, err := f.parsePlacement(raw, sc)
	if err != nil {
		return err
	}

	// Persons: identical entries merge; remap[i] is the person index of entry i.
	type person struct {
		kind    string
		objects []core.ObjectRef
	}
	var people []person
	remap := make([]int, len(out.Persons))
	for i, p := range out.Persons {
		j := slices.IndexFunc(people, func(q person) bool { return q.kind == p.Kind && sameObjects(q.objects, p.Objects) })
		if j < 0 {
			people = append(people, person{kind: p.Kind, objects: slices.Clone(p.Objects)})
			j = len(people) - 1
		}
		remap[i] = j
	}

	// References: identical type and objects merge, with the union of the owners
	// (one entry per person, the first lien kept).
	type owner struct {
		person int
		lien   string
	}
	type reference struct {
		kind    string
		objects []core.ObjectRef
		owners  []owner
	}
	var refs []reference
	for _, r := range out.FinancialRefs {
		j := slices.IndexFunc(refs, func(q reference) bool { return q.kind == r.RefKind && sameObjects(q.objects, r.Objects) })
		if j < 0 {
			refs = append(refs, reference{kind: r.RefKind, objects: slices.Clone(r.Objects)})
			j = len(refs) - 1
		}
		for _, o := range r.Owners {
			p := remap[*o.PersonIndex]
			if !slices.ContainsFunc(refs[j].owners, func(x owner) bool { return x.person == p }) {
				refs[j].owners = append(refs[j].owners, owner{person: p, lien: o.Lien})
			}
		}
	}

	// Transactions: one per transaction object; its sources are that object and
	// the selected objects linked to it that are objects of a declared financial
	// reference (its accounts).
	var txs [][]core.ObjectRef
	var seen []core.ObjectRef
	for _, t := range out.Transactions {
		if slices.Contains(seen, t.Object) {
			continue
		}
		seen = append(seen, t.Object)
		objs := []core.ObjectRef{t.Object}
		for _, linked := range sc.Linked(t.Object) {
			if !slices.Contains(objs, linked) &&
				slices.ContainsFunc(refs, func(r reference) bool { return slices.Contains(r.objects, linked) }) {
				objs = append(objs, linked)
			}
		}
		txs = append(txs, objs)
	}

	items := make([]core.Path, len(people))
	for i, p := range people {
		item := doc.AddItem(core.Path("personnes."+p.kind), p.kind)
		doc.ItemSources[item] = p.objects
		items[i] = item
	}
	for _, r := range refs {
		for _, o := range r.owners {
			container := items[o.person].Child("referencesFinancieres").Child(referenceContainerPrefix + r.kind)
			item := doc.AddItem(container, referenceContainerPrefix+r.kind)
			doc.ItemSources[item] = slices.Clone(r.objects)
			doc.Values[item.Child("lien")] = core.Value{Value: liens[o.lien], Origin: core.OriginLLM, Confidence: core.ConfidenceMedium}
		}
	}
	for _, objs := range txs {
		item := doc.AddItem(transactionsContainer, "transactions")
		doc.ItemSources[item] = slices.Clone(objs)
	}
	return nil
}

func checkObjects(objs []core.ObjectRef) error {
	if len(objs) == 0 {
		return errors.New("at least one source object is required")
	}
	for _, o := range objs {
		if strings.TrimSpace(o.Table) == "" || strings.TrimSpace(o.ObjectID) == "" {
			return errors.New("source object without table or identifier")
		}
	}
	return nil
}
