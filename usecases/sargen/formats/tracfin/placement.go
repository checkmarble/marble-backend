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
   "kind":{"type":"string","enum":["pp","pm","pnc"],"description":"pp = personne physique, pm = personne morale, pnc = personne non caractérisée"},
   "objects":{"type":"array","items":` + obj + `}}}},
 "financial_refs":{"type":"array","items":{"type":"object","additionalProperties":false,"required":["ref_kind","owners","objects"],"properties":{
   "ref_kind":{"type":"string","enum":[` + strings.Join(enum, ",") + `]},
   "owners":{"type":"array","items":{"type":"object","additionalProperties":false,"required":["person_index","lien"],"properties":{
     "person_index":{"type":"integer","description":"position de la personne dans persons (à partir de 0)"},
     "lien":{"type":"string","enum":["titulaire","co-titulaire","mandataire"]}}}},
   "objects":{"type":"array","items":` + obj + `}}}},
 "transactions":{"type":"array","items":{"type":"object","additionalProperties":false,"required":["object"],"properties":{"object":{"description":"l'objet transaction (jamais un objet de personne ni de compte déclaré) ; les comptes liés sont ajoutés par le système","type":"object","additionalProperties":false,"required":["table","object_id"],"properties":{"table":{"type":"string"},"object_id":{"type":"string"}}}}}}}}`
	var s jsonschema.Schema
	if err := json.Unmarshal([]byte(raw), &s); err != nil {
		panic("placement schema: " + err.Error()) // constant above: a programming error
	}
	return s
}

// parsePlacement decodes and checks a placement output. References without a
// valid declared owner are returned apart (dropped, reported as a question); any
// other error refuses the whole output.
func (f *format) parsePlacement(raw json.RawMessage) (out placementOutput, dropped []droppedRef, err error) {
	if err := json.NewDecoder(bytes.NewReader(raw)).Decode(&out); err != nil {
		return out, nil, errors.Wrap(err, "sortie de placement illisible")
	}
	refKinds := f.referenceKinds()
	for i, p := range out.Persons {
		if !slices.Contains(personKinds, p.Kind) {
			return out, nil, errors.Newf("personne %d : type %q inconnu (pp, pm ou pnc)", i, p.Kind)
		}
		if err := checkObjects(p.Objects); err != nil {
			return out, nil, errors.Wrapf(err, "personne %d", i)
		}
	}
	kept := out.FinancialRefs[:0:0]
	for i, r := range out.FinancialRefs {
		if !slices.Contains(refKinds, r.RefKind) {
			return out, nil, errors.Newf("référence financière %d : ref_kind %q inconnu (%s)", i, r.RefKind, strings.Join(refKinds, ", "))
		}
		if err := checkObjects(r.Objects); err != nil {
			return out, nil, errors.Wrapf(err, "référence financière %d", i)
		}
		owners := r.Owners[:0:0]
		var seen []int
		lost := 0
		for _, o := range r.Owners {
			if o.PersonIndex == nil || *o.PersonIndex < 0 || *o.PersonIndex >= len(out.Persons) {
				lost++
				continue
			}
			if _, ok := liens[o.Lien]; !ok {
				lost++
				continue
			}
			if slices.Contains(seen, *o.PersonIndex) {
				continue
			}
			seen = append(seen, *o.PersonIndex)
			owners = append(owners, o)
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
	for i, t := range out.Transactions {
		if err := checkObjects([]core.ObjectRef{t.Object}); err != nil {
			return out, nil, errors.Wrapf(err, "transaction %d", i)
		}
	}
	return out, dropped, nil
}

// droppedRef is a financial reference left out (wholly: no valid owner) or
// declared with fewer owners than listed.
type droppedRef struct {
	objects    []core.ObjectRef
	wholly     bool
	lostOwners int
}

// StructuredQuestions reports what a placement output left out: the financial
// references without a valid declared owner (core.StructuredQuestioner).
func (f *format) StructuredQuestions(kind string, raw json.RawMessage, applied *core.Document) []core.QuestionDraft {
	if kind != StepPlacement {
		return nil
	}
	_, dropped, err := f.parsePlacement(raw)
	if err != nil {
		return nil
	}
	var out []core.QuestionDraft
	for _, d := range dropped {
		names := make([]string, len(d.objects))
		for i, o := range d.objects {
			names[i] = o.Table + "/" + o.ObjectID
		}
		list := strings.Join(names, ", ")
		if d.wholly {
			out = append(out, core.QuestionDraft{Text: "référence financière sans titulaire déclaré : " + list})
		} else {
			out = append(out, core.QuestionDraft{Text: fmt.Sprintf("référence financière : %d titulaire(s) non déclaré(s) ignoré(s) : %s", d.lostOwners, list)})
		}
	}
	out = append(out, f.mixedUseQuestions(raw, applied)...)
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

// ApplyStructured applies the output of the placement step: it creates the
// person items (with their source objects), the financial references under each
// declared owner (a joint account is created under each of them, with its own
// lien, the elements sharing the same sources) and the working transactions.
//
// Dedupe: persons with the same type and the same objects are one person;
// references of the same type and objects are one reference (their owners are
// merged); transactions are identified by their transaction object (one id per
// entry; it may not be a person or reference object, see parsePlacement). An object shared by
// two distinct persons stays allowed.
//
// A transaction's sources are its objects plus the selected objects linked to
// its transaction object by the data model that are the objects of a declared
// financial reference (its account): the model does not have to list them.
//
// A reference without a valid declared owner is dropped; any other invalid
// index, reference type, lien or object is an error and the caller keeps its
// copy of the document untouched.
func (f *format) ApplyStructured(kind string, doc *core.Document, raw json.RawMessage, sc core.StructuredContext) error {
	if kind != StepPlacement {
		return errors.Newf("sortie structurée inconnue : %q", kind)
	}
	out, _, err := f.parsePlacement(raw)
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
		return errors.New("au moins un objet source est obligatoire")
	}
	for _, o := range objs {
		if strings.TrimSpace(o.Table) == "" || strings.TrimSpace(o.ObjectID) == "" {
			return errors.New("objet source sans table ou sans identifiant")
		}
	}
	return nil
}
