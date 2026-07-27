// usecases/rule_ai_description_test.go
package usecases

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/models/ast"
)

func TestPreviousCommittedRulesByStableId(t *testing.T) {
	t.Run("no committed iteration returns empty map", func(t *testing.T) {
		draftOnly := []models.ScenarioIteration{
			{Id: "draft", Version: nil, Rules: []models.Rule{{Id: "r1", StableRuleId: "s1"}}},
		}
		result := previousCommittedRulesByStableId(draftOnly)
		assert.Empty(t, result)
	})

	t.Run("picks the highest versioned committed iteration", func(t *testing.T) {
		v1 := 1
		v2 := 2
		iterations := []models.ScenarioIteration{
			{Id: "it-v1", Version: &v1, Rules: []models.Rule{{Id: "r1", StableRuleId: "s1", Name: "old"}}},
			{Id: "it-v2", Version: &v2, Rules: []models.Rule{{Id: "r2", StableRuleId: "s1", Name: "new"}}},
			{Id: "draft", Version: nil, Rules: []models.Rule{{Id: "r3", StableRuleId: "s1", Name: "draft"}}},
		}
		result := previousCommittedRulesByStableId(iterations)
		assert.Equal(t, "r2", result["s1"].Id)
		assert.Equal(t, "new", result["s1"].Name)
	})
}

func TestRulesNeedingAiDescriptionGeneration(t *testing.T) {
	nodeA := &ast.Node{Constant: "a"}
	nodeB := &ast.Node{Constant: "b"}

	t.Run("new rule (no matching stable id) needs generation", func(t *testing.T) {
		current := []models.Rule{{Id: "r1", StableRuleId: "new-stable-id", FormulaAstExpression: nodeA}}
		previous := map[string]models.Rule{}
		assert.Equal(t, []string{"r1"}, rulesNeedingAiDescriptionGeneration(current, previous))
	})

	t.Run("changed formula needs generation", func(t *testing.T) {
		current := []models.Rule{{Id: "r1", StableRuleId: "s1", FormulaAstExpression: nodeB}}
		previous := map[string]models.Rule{
			"s1": {Id: "r0", StableRuleId: "s1", FormulaAstExpression: nodeA},
		}
		assert.Equal(t, []string{"r1"}, rulesNeedingAiDescriptionGeneration(current, previous))
	})

	t.Run("unchanged formula does not need generation", func(t *testing.T) {
		current := []models.Rule{{Id: "r1", StableRuleId: "s1", FormulaAstExpression: nodeA}}
		previous := map[string]models.Rule{
			"s1": {Id: "r0", StableRuleId: "s1", FormulaAstExpression: nodeA},
		}
		assert.Empty(t, rulesNeedingAiDescriptionGeneration(current, previous))
	})

	t.Run("mix of changed and unchanged rules", func(t *testing.T) {
		current := []models.Rule{
			{Id: "r1", StableRuleId: "s1", FormulaAstExpression: nodeA},
			{Id: "r2", StableRuleId: "s2", FormulaAstExpression: nodeB},
		}
		previous := map[string]models.Rule{
			"s1": {Id: "r0", StableRuleId: "s1", FormulaAstExpression: nodeA},
			"s2": {Id: "r00", StableRuleId: "s2", FormulaAstExpression: nodeA},
		}
		assert.Equal(t, []string{"r2"}, rulesNeedingAiDescriptionGeneration(current, previous))
	})
}
