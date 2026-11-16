package tracfin_test

import (
	"regexp"
	"strings"
	"testing"

	"github.com/checkmarble/marble-backend/usecases/sargen/core"
	"github.com/checkmarble/marble-backend/usecases/sargen/formats/tracfin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func currentSpec(t *testing.T) *core.Spec {
	t.Helper()
	s, ok := tracfin.Spec(tracfin.CurrentVersion)
	require.True(t, ok)
	return s
}

func TestSpec_TemplatesAreUniqueAndParseable(t *testing.T) {
	s := currentSpec(t)
	seen := map[string]bool{}
	walkNodes(s.Root, func(n *core.Node) {
		assert.False(t, seen[n.Template], "gabarit en double : %s", n.Template)
		seen[n.Template] = true
		_, err := core.ParsePath(strings.ReplaceAll(n.Template, "[]", "[x]"))
		assert.NoError(t, err, "gabarit non lisible par ParsePath : %s", n.Template)
		assert.Same(t, n, s.NodeAt(n.Template), "NodeAt ne retrouve pas %s", n.Template)
	})
}

func TestSpec_PatternsCompileInGo(t *testing.T) {
	s := currentSpec(t)
	walkNodes(s.Root, func(n *core.Node) {
		if n.Pattern == "" {
			return
		}
		_, err := regexp.Compile("^(?:" + n.Pattern + ")$")
		assert.NoError(t, err, "motif illisible par Go : %s (%s)", n.Template, n.Pattern)
	})
	assert.NotContains(t, s.NodeAt("organisme.adresse.tel").Pattern, "^")
	assert.NotContains(t, s.NodeAt("organisme.adresse.courriel").Pattern, "$")
}

func TestSpec_CodeListsAndRefsResolve(t *testing.T) {
	s := currentSpec(t)
	walkNodes(s.Root, func(n *core.Node) {
		if n.Type == core.TypeCode {
			_, ok := s.CodeLists[n.CodeList]
			assert.True(t, ok, "liste de codes absente pour %s : %q", n.Template, n.CodeList)
		}
		if n.Type == core.TypeRef {
			assert.NotEmpty(t, n.RefTargets, n.Template)
			for _, target := range n.RefTargets {
				assert.NotNil(t, s.NodeAt(target), "cible de ref introuvable : %s -> %s", n.Template, target)
			}
		}
	})
}

func TestSpec_RulesResolve(t *testing.T) {
	s := currentSpec(t)
	ids := map[string]bool{}
	for _, rule := range s.Rules {
		assert.False(t, ids[rule.ID], "identifiant de règle en double : %s", rule.ID)
		ids[rule.ID] = true
		assert.NotEmpty(t, rule.Description, rule.ID)
		for _, path := range append(append([]string{}, rule.Then...), rule.Among...) {
			assert.NotNil(t, s.NodeAt(path), "%s : gabarit introuvable %s", rule.ID, path)
		}
		if rule.When != nil {
			assert.NotNil(t, s.NodeAt(rule.When.Path), "%s : gabarit introuvable %s", rule.ID, rule.When.Path)
		}
	}
}

func TestSpec_ExportIDsAndDefaults(t *testing.T) {
	s := currentSpec(t)

	for _, template := range []string{
		"personnes.pp[].id",
		"personnes.pm[].id",
		"personnes.pnc[].id",
		"personnes.pp[].referencesFinancieres.referenceFinanciereIban[].id",
		"personnes.pnc[].referencesFinancieres.referenceFinanciereAutre[].id",
	} {
		node := s.NodeAt(template)
		require.NotNil(t, node, template)
		assert.True(t, node.ExportID, template)
	}
	assert.True(t, s.NodeAt("personnes.pp[].referencesFinancieres.referenceFinanciereIban[].id").Attribute)
	assert.False(t, s.NodeAt("personnes.pp[].implication").ExportID)

	for template := range s.Defaults {
		assert.NotNil(t, s.NodeAt(template), template)
	}
	assert.Equal(t, "false", s.Defaults["context.general.estRequisitionJudiciaire"])
	assert.Equal(t, []string{"DS_INITIALE"}, s.ReportTypes)
}

func TestSpec_OutOfScopeSectionsAreDropped(t *testing.T) {
	s := currentSpec(t)
	for _, template := range []string{"operations.operationEnCours[]", "operationstdf", "pieceJointe"} {
		assert.Nil(t, s.NodeAt(template), template)
	}
	walkNodes(s.Root, func(n *core.Node) {
		assert.NotContains(t, n.Name, "idPieceJointe", n.Template)
	})
}

func TestSpec_DromcomCountryListIncludesBothSources(t *testing.T) {
	s := currentSpec(t)
	node := s.NodeAt("operations.operation[].paysOrigineFonds")
	require.NotNil(t, node)
	list := s.CodeLists[node.CodeList]
	values := map[string]bool{}
	for _, c := range list.Codes {
		values[c.Value] = true
	}
	assert.True(t, values["GP"], "code DROM-COM attendu")
	assert.True(t, values["AF"], "code pays attendu")
}
