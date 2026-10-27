package tracfin

import (
	"bytes"
	"encoding/xml"
	"strings"
	"testing"

	"github.com/checkmarble/marble-backend/models"
)

func task5Reference(id, target, person string) models.TracfinValue {
	return models.TracfinValue{ID: id, Kind: "reference", Reference: &models.TracfinReference{BlockID: target, PersonBlockID: person}}
}

type task5ReferenceXML struct {
	People struct {
		PP []struct {
			ID    string `xml:"id"`
			Civil struct {
				Name string `xml:"nomNaissance"`
			} `xml:"etatCivil"`
			References struct {
				IBAN []struct {
					ID string `xml:"id,attr"`
				} `xml:"referenceFinanciereIban"`
			} `xml:"referencesFinancieres"`
		} `xml:"pp"`
		PM []struct {
			ID       string `xml:"id"`
			Director struct {
				Person      string `xml:"idPersonne"`
				Beneficiary string `xml:"idBeneficiaireEffectif"`
			} `xml:"dirigeant"`
		} `xml:"pm"`
		PNC []struct {
			ID   string `xml:"id"`
			Link struct {
				Person string `xml:"idPersonne"`
			} `xml:"lienAutrePersonneDejaDeclaree"`
		} `xml:"pnc"`
	} `xml:"personnes"`
	Operations struct {
		Main []struct {
			Emitter string `xml:"emetteurTypeEmetteur"`
			Support string `xml:"refFinanciereEmission"`
		} `xml:"operation"`
		Ongoing []struct {
			Emitter string `xml:"identEmetteur"`
			Support string `xml:"refFinanciere"`
		} `xml:"operationEnCours"`
	} `xml:"operations"`
}

func task5ReferenceDraft(c Catalog) models.TracfinDraft {
	const pp = "/ds/personnes/pp"
	const pm = "/ds/personnes/pm"
	const pnc = "/ds/personnes/pnc"
	const op = "/ds/operations/operation"
	const ongoing = "/ds/operations/operationEnCours"
	support := task5Block("support", pp+"/referencesFinancieres/referenceFinanciereIban")
	civil := task5Block("civil-one", pp+"/etatCivil")
	civil.Fields[pp+"/etatCivil/nomNaissance"] = []models.TracfinValue{task5Text("name-one", "One")}
	person := task5Block("person-one", pp, civil, task5Block("supports", pp+"/referencesFinancieres", support))
	civil2 := task5Block("civil-two", pp+"/etatCivil")
	civil2.Fields[pp+"/etatCivil/nomNaissance"] = []models.TracfinValue{task5Text("name-two", "Two")}
	person2 := task5Block("person-two", pp, civil2)
	director := task5Block("director", pm+"/dirigeant")
	director.Fields[pm+"/dirigeant/idPersonne"] = []models.TracfinValue{task5Reference("director-link", "person-one", "")}
	director.Fields[pm+"/dirigeant/idBeneficiaireEffectif"] = []models.TracfinValue{task5Reference("beneficiary-link", "person-one", "")}
	company := task5Block("company", pm, director)
	link := task5Block("pnc-link", pnc+"/lienAutrePersonneDejaDeclaree")
	link.Fields[pnc+"/lienAutrePersonneDejaDeclaree/idPersonne"] = []models.TracfinValue{task5Reference("pnc-person", "person-one", "")}
	other := task5Block("other", pnc, link)
	main := task5Block("main", op)
	main.Fields[op+"/emetteurTypeEmetteur"] = []models.TracfinValue{task5Reference("main-person", "person-one", "")}
	main.Fields[op+"/refFinanciereEmission"] = []models.TracfinValue{task5Reference("main-support", "support", "person-one")}
	future := task5Block("future", ongoing)
	future.Fields[ongoing+"/identEmetteur"] = []models.TracfinValue{task5Reference("future-person", "person-one", "")}
	future.Fields[ongoing+"/refFinanciere"] = []models.TracfinValue{task5Reference("future-support", "support", "person-one")}
	return task5Draft(c, task5Block("people", "/ds/personnes", person, person2, company, other), task5Block("operations", "/ds/operations", main, future))
}

func task5ReadReferences(t *testing.T, d models.TracfinDraft, c Catalog) task5ReferenceXML {
	t.Helper()
	var b bytes.Buffer
	_, err := WriteXML(&b, d, nil, c)
	if err != nil {
		t.Fatal(err)
	}
	var result task5ReferenceXML
	if err := xml.Unmarshal(b.Bytes(), &result); err != nil {
		t.Fatalf("%v: %s", err, b.String())
	}
	return result
}
func task5CheckReferenceXML(t *testing.T, x task5ReferenceXML) (string, string) {
	t.Helper()
	if len(x.People.PP) != 2 || len(x.People.PM) != 1 || len(x.People.PNC) != 1 || len(x.Operations.Main) != 1 || len(x.Operations.Ongoing) != 1 {
		t.Fatalf("missing declared objects: %+v", x)
	}
	var person, other, support string
	for _, p := range x.People.PP {
		if p.Civil.Name == "One" {
			person = p.ID
			if len(p.References.IBAN) == 1 {
				support = p.References.IBAN[0].ID
			}
		} else if p.Civil.Name == "Two" {
			other = p.ID
		}
	}
	if person == "" || other == "" || person == other || support == "" || x.People.PM[0].ID == "" || x.People.PNC[0].ID == "" {
		t.Fatalf("missing or duplicate local identifiers: %+v", x)
	}
	for _, v := range []string{x.People.PM[0].Director.Person, x.People.PM[0].Director.Beneficiary, x.People.PNC[0].Link.Person, x.Operations.Main[0].Emitter, x.Operations.Ongoing[0].Emitter} {
		if v != person {
			t.Fatalf("identity link %q != %q: %+v", v, person, x)
		}
	}
	if x.Operations.Main[0].Support != support || x.Operations.Ongoing[0].Support != support {
		t.Fatalf("support links not owned by selected person: %+v", x)
	}
	return person, support
}

func TestTracfinDeterministicReferences(t *testing.T) {
	c := task5Catalog(t)
	d := task5ReferenceDraft(c)
	p, s := task5CheckReferenceXML(t, task5ReadReferences(t, d, c))
	children := d.Document.Root.Children[0].Children
	children[0], children[1] = children[1], children[0]
	p2, s2 := task5CheckReferenceXML(t, task5ReadReferences(t, d, c))
	if p != p2 || s != s2 {
		t.Fatalf("stable IDs changed after reorder: person %q -> %q, support %q -> %q", p, p2, s, s2)
	}
}

func TestTracfinOngoingReferenceKinds(t *testing.T) {
	c := task5Catalog(t)
	const base = "/ds/operations/operationEnCours"
	for _, name := range []string{"identEmetteur", "refFinanciere"} {
		f := c.Fields[base+"/"+name]
		if f.ValueKind != "reference" {
			t.Errorf("%s is %q, expected reference", f.ID, f.ValueKind)
		}
		b := task5Block("future", base)
		b.Fields[f.ID] = []models.TracfinValue{task5Text(name, "12345")}
		d := task5Draft(c, task5Block("operations", "/ds/operations", b))
		if !task5HasField(ValidateDraft(d, c).Diagnostics, f.ID) {
			t.Errorf("numeric text accepted as ongoing reference %s", f.ID)
		}
	}
}

func TestTracfinXMLIndependentReferences(t *testing.T) {
	c := task5Catalog(t)
	// These are literal XML documents, independent of WriteXML; missing mandatory unrelated fields may still be diagnosed.
	const head = `<ds><personnes><pp><id>1</id><etatCivil></etatCivil><referencesFinancieres><referenceFinanciereIban id="10"></referenceFinanciereIban></referencesFinancieres></pp><pp><id>2</id><etatCivil></etatCivil></pp></personnes><operations><operationEnCours>`
	const tail = `</operationEnCours></operations></ds>`
	for _, tc := range []struct {
		name, body string
		fail       bool
	}{{"same person", `<identEmetteur>1</identEmetteur><refFinanciere>10</refFinanciere>`, false}, {"dangling person", `<identEmetteur>99</identEmetteur>`, true}, {"dangling support", `<identEmetteur>1</identEmetteur><refFinanciere>99</refFinanciere>`, true}, {"wrong owner", `<identEmetteur>2</identEmetteur><refFinanciere>10</refFinanciere>`, true}} {
		t.Run(tc.name, func(t *testing.T) {
			v := ValidateXML(strings.NewReader(head+tc.body+tail), c)
			if (v.Layers["references"].Status == "failed") != tc.fail {
				t.Fatalf("reference result: %+v", v)
			}
		})
	}
}
