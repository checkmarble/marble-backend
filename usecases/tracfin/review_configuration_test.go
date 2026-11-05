package tracfin

import (
	"testing"

	"github.com/checkmarble/marble-backend/models"
	"github.com/stretchr/testify/require"
)

// Mappings sharing a root table share their selection: a step ID must denote
// the same hop in all of them, or every selection on that root is unusable.
func TestTracfinConfigurationConsistentStepsAcrossMappings(t *testing.T) {
	c := draftCatalog(t)
	account := models.TracfinPathStep{ID: "account", ParentID: "root", Table: "Account", Link: "owner", Direction: "inverse"}
	other := account
	other.Link = "holder"
	mapping := func(id, target, table string, step models.TracfinPathStep) models.TracfinMapping {
		return models.TracfinMapping{ID: id, TargetField: target, Source: models.TracfinMappingSource{Kind: "ingested", EntityType: "Person", Table: table, Field: "v", Path: []models.TracfinPathStep{step}}}
	}
	const civil = "/ds/personnes/pp/etatCivil"
	conflict := ValidateConfiguration(models.TracfinConfiguration{Mappings: []models.TracfinMapping{mapping("a", civil+"/nomNaissance", "User", account), mapping("b", civil+"/prenom", "User", other)}}, c)
	require.Len(t, conflict, 1)
	require.Equal(t, "INCONSISTENT_MAPPING_PATH", conflict[0].Code)
	require.Empty(t, ValidateConfiguration(models.TracfinConfiguration{Mappings: []models.TracfinMapping{mapping("a", civil+"/nomNaissance", "User", account), mapping("b", civil+"/prenom", "User", account)}}, c))
	require.Empty(t, ValidateConfiguration(models.TracfinConfiguration{Mappings: []models.TracfinMapping{mapping("a", civil+"/nomNaissance", "User", account), mapping("b", civil+"/prenom", "Client", other)}}, c), "different roots have independent selections")
}
