package tracfin

import (
	"bytes"
	"encoding/json"
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
 "transactions":{"type":"array","items":{"type":"object","additionalProperties":false,"required":["object"],"properties":{"object":` + obj + `}}}}}`
	var s jsonschema.Schema
	if err := json.Unmarshal([]byte(raw), &s); err != nil {
		panic("placement schema: " + err.Error()) // constant above: a programming error
	}
	return s
}

// ApplyStructured applies the output of the placement step: it creates the
// person items (with their source objects), the financial references under each
// declared owner (a joint account is created under each of them, with its own
// lien, the elements sharing the same sources) and the working transactions.
// Any invalid index, reference type, lien or object is an error and nothing
// should be kept: the caller applies it to a copy of the document.
func (f *format) ApplyStructured(kind string, doc *core.Document, raw json.RawMessage) error {
	if kind != StepPlacement {
		return errors.Newf("sortie structurée inconnue : %q", kind)
	}
	var out placementOutput
	dec := json.NewDecoder(bytes.NewReader(raw))
	if err := dec.Decode(&out); err != nil {
		return errors.Wrap(err, "sortie de placement illisible")
	}
	refKinds := f.referenceKinds()

	// Validate everything first so that a refused output leaves doc untouched.
	for i, p := range out.Persons {
		if !slices.Contains(personKinds, p.Kind) {
			return errors.Newf("personne %d : type %q inconnu (pp, pm ou pnc)", i, p.Kind)
		}
		if err := checkObjects(p.Objects); err != nil {
			return errors.Wrapf(err, "personne %d", i)
		}
	}
	for i, r := range out.FinancialRefs {
		if !slices.Contains(refKinds, r.RefKind) {
			return errors.Newf("référence financière %d : ref_kind %q inconnu (%s)", i, r.RefKind, strings.Join(refKinds, ", "))
		}
		if err := checkObjects(r.Objects); err != nil {
			return errors.Wrapf(err, "référence financière %d", i)
		}
		if len(r.Owners) == 0 {
			return errors.Newf("référence financière %d : aucun titulaire déclaré", i)
		}
		for _, o := range r.Owners {
			if o.PersonIndex == nil || *o.PersonIndex < 0 || *o.PersonIndex >= len(out.Persons) {
				return errors.Newf("référence financière %d : person_index invalide", i)
			}
			if _, ok := liens[o.Lien]; !ok {
				return errors.Newf("référence financière %d : lien %q inconnu (titulaire, co-titulaire ou mandataire)", i, o.Lien)
			}
		}
	}
	for i, t := range out.Transactions {
		if err := checkObjects([]core.ObjectRef{t.Object}); err != nil {
			return errors.Wrapf(err, "transaction %d", i)
		}
	}

	persons := make([]core.Path, len(out.Persons))
	for i, p := range out.Persons {
		item := doc.AddItem(core.Path("personnes."+p.Kind), p.Kind)
		doc.ItemSources[item] = slices.Clone(p.Objects)
		persons[i] = item
	}
	for _, r := range out.FinancialRefs {
		for _, o := range r.Owners {
			container := persons[*o.PersonIndex].Child("referencesFinancieres").Child(referenceContainerPrefix + r.RefKind)
			item := doc.AddItem(container, referenceContainerPrefix+r.RefKind)
			doc.ItemSources[item] = slices.Clone(r.Objects)
			doc.Values[item.Child("lien")] = core.Value{Value: liens[o.Lien], Origin: core.OriginLLM, Confidence: core.ConfidenceMedium}
		}
	}
	for _, t := range out.Transactions {
		item := doc.AddItem(transactionsContainer, "transactions")
		doc.ItemSources[item] = []core.ObjectRef{t.Object}
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
