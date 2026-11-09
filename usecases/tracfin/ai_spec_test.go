package tracfin

import (
	"encoding/json"
	"testing"

	"github.com/checkmarble/marble-backend/models"
	"github.com/invopop/jsonschema"
	"github.com/stretchr/testify/require"
)

func TestAISpecDescribesExpectedFormats(t *testing.T) {
	c := task5Catalog(t)
	birth := c.AISpec("/ds/personnes/pp/etatCivil/dateNaissance")
	require.Equal(t, "field", birth.Kind)
	require.Equal(t, "/ds/personnes/pp/etatCivil", birth.OwnerBlock)
	require.Contains(t, birth.Format, "DD/MM/YYYY")

	register := c.AISpec("/ds/personnes/pm/activiteCoordonnee/numeroIdentification/@type")
	require.Equal(t, "/ds/personnes/pm/activiteCoordonnee", register.OwnerBlock)
	require.Equal(t, "/ds/personnes/pm/activiteCoordonnee/numeroIdentification", register.ParentScalar)
	require.Contains(t, register.Allowed, "Je ne sais pas")
	require.Contains(t, register.Allowed, "SIREN")

	block := c.AISpec("/ds/personnes/pm/activiteCoordonnee")
	require.Equal(t, "block", block.Kind)
	require.Empty(t, block.Format)
	require.Contains(t, block.Help, "RN1")

	amount := c.AISpec("/ds/operations/operation/montantTotal")
	require.Contains(t, amount.Format, "12")
	require.Contains(t, c.AISpec("/ds/personnes/pp/etatCivil/nationalite").CodeLists, "pays.csv")
}

func TestAIValidationMessagesStateTheExpectation(t *testing.T) {
	c := task5Catalog(t)
	civil := task5Civil(map[string][]models.TracfinValue{
		"/ds/personnes/pp/etatCivil/dateNaissance": {task5Text("birth", "1985-07-14")},
	})
	report := ValidateDraft(task5Draft(c, civil), c)
	found := false
	for _, d := range report.Diagnostics {
		if d.Field != nil && d.Field.FieldID == "/ds/personnes/pp/etatCivil/dateNaissance" && d.Severity == "error" {
			found = true
			require.Contains(t, d.Message, "DD/MM/YYYY", "%+v", d)
		}
	}
	require.True(t, found, "%+v", report.Diagnostics)
}

func TestAIApplicationErrorsNameProposalKeys(t *testing.T) {
	c := task5Catalog(t)
	p := AISectionProposal{
		Blocks: []AIBlock{{Key: "civil-claire", ParentKey: "pp", CatalogID: "/ds/personnes/pp/etatCivil"}},
		Values: []AIValue{scopeValue("birth-claire", "civil-claire", "/ds/personnes/pp/etatCivil/dateNaissance", "1985-07-14")},
	}
	d := task5Draft(c, task5Block("people", "/ds/personnes", task5Block("pp", "/ds/personnes/pp")))
	_, ds := ApplyAISection(d, "persons", p, map[string]bool{"s": true}, c, "run")
	var target *models.TracfinDiagnostic
	for i := range ds {
		if ds[i].Severity == "error" && ds[i].Field != nil && ds[i].Field.FieldID == "/ds/personnes/pp/etatCivil/dateNaissance" {
			target = &ds[i]
		}
	}
	require.NotNil(t, target, "%+v", ds)
	require.Contains(t, target.Message, "block_key civil-claire")
	require.Contains(t, target.Message, "value key birth-claire")
}

func TestAIProposalSchemaCarriesContract(t *testing.T) {
	raw, err := json.Marshal(jsonschema.Reflect(AISectionProposal{}))
	require.NoError(t, err)
	var schema struct {
		Defs map[string]struct {
			Properties map[string]struct {
				Description string `json:"description"`
				Enum        []any  `json:"enum"`
			} `json:"properties"`
		} `json:"$defs"`
	}
	require.NoError(t, json.Unmarshal(raw, &schema))
	value := schema.Defs["AIValue"].Properties
	require.ElementsMatch(t, []any{"text", "boolean", "decimal", "reference"}, value["kind"].Enum)
	require.Contains(t, value["field_id"].Description, "kind field")
	require.Contains(t, schema.Defs["TracfinAIEvidence"].Properties["locator"].Description, "JSON Pointer")
}
