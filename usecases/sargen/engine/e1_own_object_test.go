package engine

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/usecases/sargen/core"
)

// ownObjectFixture seeds a legal entity whose sources are its own row (companies/c1)
// and the account it points to (accounts/a1, the parent of a belongs-to link held
// by the company row). The account has an IBAN, the company a country.
func ownObjectFixture(t *testing.T, company map[string]any, withLink bool) (*Orchestrator, *fakes) {
	t.Helper()
	o, f := newTestOrchestrator(t)
	node := f.Format.Spec().NodeAt("personnes.pm[].activiteCoordonnee.paysAutreTypeRegistre")
	require.NotNil(t, node)
	seedDocument(t, f, func(d *core.Document) {
		withForeignRegister("")(d)
		d.ItemSources["personnes.pm[pm-1]"] = []core.ObjectRef{{Table: "companies", ObjectID: "c1"}, {Table: "accounts", ObjectID: "a1"}}
	})
	company["object_id"] = "c1"
	company["account_id"] = "a1"
	f.Repo.selection = []models.SarSelectedObject{
		agentSelection("companies", "c1", company),
		agentSelection("accounts", "a1", map[string]any{"object_id": "a1", "iban": "FR7630006000011234567890189"}),
	}
	cf := map[string]models.Field{"object_id": {Name: "object_id", DataType: models.String}, "account_id": {Name: "account_id", DataType: models.String}}
	for k := range company {
		if k != "object_id" && k != "account_id" {
			cf[k] = models.Field{Name: k, DataType: models.String, SemanticType: models.FieldSemanticTypeCountry}
		}
	}
	company2 := models.Table{Name: "companies", Fields: cf}
	if withLink {
		company2.LinksToSingle = map[string]models.LinkToSingle{"account": {
			Name: "account", LinkType: models.LinkTypeBelongsTo,
			ParentTableName: "accounts", ParentFieldName: "object_id", ChildTableName: "companies", ChildFieldName: "account_id",
		}}
	}
	f.Repo.dataModel = models.DataModel{Tables: map[string]models.Table{
		"companies": company2,
		"accounts": {Name: "accounts", Fields: map[string]models.Field{
			"object_id": {Name: "object_id", DataType: models.String},
			"iban":      {Name: "iban", DataType: models.String, SemanticType: models.FieldSemanticTypeIban},
		}},
	}}
	f.Format.CodeSemantics = map[string][]models.FieldSemanticType{node.CodeList: {models.FieldSemanticTypeCountry, models.FieldSemanticTypeIban}}
	f.Format.CodeFromValueFn = func(raw string) (string, bool) {
		switch raw {
		case "HK":
			return "HK", true
		case "NZ":
			return "NZ", true
		case "FR7630006000011234567890189":
			return "FR", true
		}
		return "", false
	}
	return o, f
}

// A source reached through a link (the account of the item's own row) is not
// searched: only the own row's country is offered.
func TestMissingCode_LinkedParentObjectIsNotSearched(t *testing.T) {
	o, f := ownObjectFixture(t, map[string]any{"country": "HK"}, true)
	require.NoError(t, o.RunJob(context.Background(), f.JobId))
	q, opts, ok := questionOn(t, f, pmRegistrePay)
	require.True(t, ok)
	require.Len(t, opts, 1)
	assert.Equal(t, "HK", opts[0].Fills[0].Value)
	assert.NotContains(t, q.Text, "accounts/a1")
}

// Several columns of the own row are all candidates.
func TestMissingCode_OwnRowKeepsItsCountryAndNationality(t *testing.T) {
	o, f := ownObjectFixture(t, map[string]any{"country": "HK", "nationality": "NZ"}, true)
	require.NoError(t, o.RunJob(context.Background(), f.JobId))
	_, opts, ok := questionOn(t, f, pmRegistrePay)
	require.True(t, ok)
	var got []string
	for _, op := range opts {
		got = append(got, op.Fills[0].Value)
	}
	assert.ElementsMatch(t, []string{"HK", "NZ"}, got)
}

// Nothing in the own row: the unknown code, not the linked account's IBAN.
func TestMissingCode_OnlyLinkedObjectHasAValueKeepsTheUnknownOption(t *testing.T) {
	o, f := ownObjectFixture(t, map[string]any{}, true)
	require.NoError(t, o.RunJob(context.Background(), f.JobId))
	_, opts, ok := questionOn(t, f, pmRegistrePay)
	require.True(t, ok)
	require.Len(t, opts, 1)
	assert.Equal(t, "Inconnu", opts[0].Label)
}
