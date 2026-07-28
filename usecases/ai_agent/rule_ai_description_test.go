package ai_agent

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/checkmarble/marble-backend/models"
)

func TestPreviousCommittedRulesByStableId(t *testing.T) {
	t.Run("no committed iteration returns empty map", func(t *testing.T) {
		draftOnly := []models.ScenarioIteration{
			{Id: "draft", Version: nil, Rules: []models.Rule{{Id: "r1", StableRuleId: "s1"}}},
		}
		result := PreviousCommittedRulesByStableId(draftOnly)
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
		result := PreviousCommittedRulesByStableId(iterations)
		assert.Equal(t, "r2", result["s1"].Id)
		assert.Equal(t, "new", result["s1"].Name)
	})
}

func TestRulesNeedingAiDescriptionGeneration(t *testing.T) {
	t.Run("no AI description and no user description needs generation", func(t *testing.T) {
		current := []models.Rule{{Id: "r1", AiDescription: "", Description: ""}}
		assert.Equal(t, []string{"r1"}, RulesNeedingAiDescriptionGeneration(current))
	})

	t.Run("existing AI description does not need generation", func(t *testing.T) {
		current := []models.Rule{{Id: "r1", AiDescription: "description", Description: ""}}
		assert.Empty(t, RulesNeedingAiDescriptionGeneration(current))
	})

	t.Run("user-authored description is never overridden, even without an AI description", func(t *testing.T) {
		current := []models.Rule{{Id: "r1", AiDescription: "", Description: "user description"}}
		assert.Empty(t, RulesNeedingAiDescriptionGeneration(current))
	})

	t.Run("user-authored description takes precedence over a missing AI description", func(t *testing.T) {
		current := []models.Rule{{Id: "r1", AiDescription: "description", Description: "user description"}}
		assert.Empty(t, RulesNeedingAiDescriptionGeneration(current))
	})

	t.Run("mix of rules preserves input order and only flags rules needing generation", func(t *testing.T) {
		current := []models.Rule{
			{Id: "r1", AiDescription: "description", Description: ""},                 // has AI description: excluded
			{Id: "r2", AiDescription: "", Description: ""},                            // needs generation: included
			{Id: "r3", AiDescription: "", Description: "user description"},            // user-authored: excluded
			{Id: "r4", AiDescription: "description", Description: "user description"}, // user-authored: excluded
			{Id: "r5", AiDescription: "", Description: ""},                            // needs generation: included
		}
		assert.Equal(t, []string{"r2", "r5"}, RulesNeedingAiDescriptionGeneration(current))
	})
}
