package tracfin

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/checkmarble/marble-backend/models"
)

func TestTracfinReviewXMLBusinessRules(t *testing.T) {
	c := task5Catalog(t)
	support := ""
	for i := 0; i < 11; i++ {
		support += `<referenceFinanciereIban id="` + fmt.Sprint(i+1) + `"/>`
	}
	for i := 0; i < 10; i++ {
		support += `<referenceFinanciereNumeroCarteBancaire id="` + fmt.Sprint(i+12) + `"/>`
	}
	for _, tc := range []struct{ name, doc, field string }{
		{"competing director alternatives", `<ds><personnes><pm><dirigeant><autreDirigeantP><nom>A</nom></autreDirigeantP><autreDirigeantM><denominationSociale>B</denominationSociale></autreDirigeantM></dirigeant></pm></personnes></ds>`, `/ds/personnes/pm/dirigeant`},
		{"other operation", `<ds><operations><operation><typeOperation>OT11</typeOperation></operation></operations></ds>`, `/ds/operations/operation/autretypeOperation`},
		{"combined financial supports", `<ds><personnes><pp><referencesFinancieres>` + support + `</referencesFinancieres></pp></personnes></ds>`, `/ds/personnes/pp/referencesFinancieres`},
		{"identity card", `<ds><pieceJointe><piece><type>TA2</type></piece></pieceJointe></ds>`, `/ds/pieceJointe/piece/infoPieceIdentite`},
		{"passport", `<ds><pieceJointe><piece><type>TA3</type></piece></pieceJointe></ds>`, `/ds/pieceJointe/piece/infoPieceIdentite`},
		{"residence permit", `<ds><pieceJointe><piece><type>TA5</type></piece></pieceJointe></ds>`, `/ds/pieceJointe/piece/infoPieceIdentite`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v := ValidateXML(strings.NewReader(tc.doc), c)
			found := false
			for _, d := range v.Diagnostics {
				if d.Layer == "business" && d.Field != nil && (d.Field.FieldID == tc.field || (tc.name == "competing director alternatives" && strings.HasPrefix(d.Field.FieldID, tc.field+"/"))) {
					found = true
				}
			}
			if !found {
				t.Fatalf("missing localized business diagnostic for %s: %+v", tc.field, v.Diagnostics)
			}
		})
	}
}

func TestTracfinReviewReferencePrefillRequiresSelection(t *testing.T) {
	for _, tc := range []struct {
		name, block, field string
		reference          bool
	}{
		{"director reference", "/ds/personnes/pm/dirigeant", "/ds/personnes/pm/dirigeant/idPersonne", true},
		{"ongoing reference", "/ds/operations/operationEnCours", "/ds/operations/operationEnCours/identEmetteur", true},
		{"principal numeric text", "/ds/operations/operation", "/ds/operations/operation/emetteurTypeEmetteur", false},
		{"ordinary text", "/ds/declarant/coordonnees", "/ds/declarant/coordonnees/nom", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := models.TracfinMapping{ID: "m", TargetField: tc.field, Source: models.TracfinMappingSource{Kind: "ingested", Table: "User", Field: "external_id"}}
			cfg := models.TracfinConfiguration{Mappings: []models.TracfinMapping{m}}
			sel := &models.TracfinSelection{MappingID: "m", Root: models.TracfinPathStep{ID: "root", Table: "User", RecordID: "U"}}
			d := models.TracfinDraft{Document: models.TracfinDocument{Root: models.TracfinBlock{ID: "target", CatalogID: tc.block, Selection: sel}}}
			src := SourceSet{Records: map[SourceKey]SourceRecord{{"target", "root"}: {Table: "User", RecordID: "U", Data: map[string]any{"external_id": "12345"}}}}
			result := ResolvePrefill(cfg, d, src)
			if tc.reference {
				if len(result.Proposals) != 0 || !task5HasField(result.Diagnostics, tc.field) {
					t.Fatalf("reference-only target proposed source identifier without selection: %+v", result)
				}
			} else if len(result.Proposals) != 1 || len(result.Proposals[0].Candidates) != 1 || result.Proposals[0].Candidates[0].Text == nil || *result.Proposals[0].Candidates[0].Text != "12345" {
				t.Fatalf("text control lost: %+v", result)
			}
		})
	}
}

func TestTracfinReviewRequiredScalarAttributes(t *testing.T) {
	c := task5Catalog(t)
	for _, tc := range []struct{ name, block, field, attr, value string }{{"register type", "/ds/personnes/pm/activiteCoordonnee", "/ds/personnes/pm/activiteCoordonnee/numeroIdentification", "type", "RE01"}, {"suspicion ue", "/ds/personnes/pp/notorieteDefavorable", "/ds/personnes/pp/notorieteDefavorable/precisionsSuspicion", "ue", "true"}} {
		t.Run(tc.name, func(t *testing.T) {
			attr := tc.field + "/@" + tc.attr
			b := task5Block("block", tc.block)
			b.Fields[tc.field] = []models.TracfinValue{task5Text("v1", "one"), task5Text("v2", "two")}
			a := task5Text("attr1", tc.value)
			a.ParentValueID = "v1"
			b.Fields[attr] = []models.TracfinValue{a}
			wrap := func(block models.TracfinBlock) models.TracfinDraft {
				person := "/ds/personnes/pp"
				if strings.Contains(block.CatalogID, "/pm/") {
					person = "/ds/personnes/pm"
				}
				return task5Draft(c, task5Block("people", "/ds/personnes", task5Block("person", person, block)))
			}
			d := wrap(b)
			hasMissing := func(ds []models.TracfinDiagnostic, owner string) bool {
				for _, x := range ds {
					if x.Field != nil && x.Field.BlockID == "block" && x.Field.FieldID == attr && x.Field.ParentValueID == owner && x.Field.ValueID == "" {
						return true
					}
				}
				return false
			}
			v := ValidateDraft(d, c)
			if !hasMissing(v.Diagnostics, "v2") || hasMissing(v.Diagnostics, "v1") {
				t.Errorf("missing attribute must identify only its owner: %+v", v.Diagnostics)
			}
			var out bytes.Buffer
			index, err := WriteXML(&out, d, nil, c)
			if err != nil {
				t.Fatal(err)
			}
			if !hasMissing(index.Diagnostics, "v2") || hasMissing(index.Diagnostics, "v1") {
				t.Errorf("export missing owned attribute diagnostic: %+v", index.Diagnostics)
			}
			a2 := task5Text("attr2", tc.value)
			a2.ParentValueID = "v2"
			b.Fields[attr] = append(b.Fields[attr], a2)
			d = wrap(b)
			if hasMissing(ValidateDraft(d, c).Diagnostics, "v2") {
				t.Error("present attribute diagnosed missing")
			}
		})
	}
}
