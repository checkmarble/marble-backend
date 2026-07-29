package ai_agent

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/checkmarble/marble-backend/models"
)

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
