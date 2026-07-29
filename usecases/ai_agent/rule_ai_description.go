package ai_agent

import "github.com/checkmarble/marble-backend/models"

// RulesNeedingAiDescriptionGeneration returns the ids of currentRules that need a new
// AI-generated description. Rules authored by user are excluded because the user's description is
// preferred over an AI generated one.
// We don't compare the current rules with the previous rules version because when updating the rule formula,
// we empty the Ai description to force the generation even if the formula stay the same.
func RulesNeedingAiDescriptionGeneration(currentRules []models.Rule) []string {
	var ruleIds []string
	for _, rule := range currentRules {
		if rule.Description != "" {
			continue
		}

		if rule.AiDescription == "" {
			ruleIds = append(ruleIds, rule.Id)
		}
	}
	return ruleIds
}
