package ai_agent

import "github.com/checkmarble/marble-backend/models"

// PreviousCommittedRulesByStableId returns, for the most recently committed
// iteration among the given ones (highest non-nil Version), its rules
// indexed by StableRuleId. Returns an empty map if none of the iterations
// is committed yet.
func PreviousCommittedRulesByStableId(iterations []models.ScenarioIteration) map[string]models.Rule {
	var latest *models.ScenarioIteration
	for i := range iterations {
		it := iterations[i]
		if it.Version == nil {
			continue
		}
		if latest == nil || *it.Version > *latest.Version {
			latest = &it
		}
	}

	result := make(map[string]models.Rule)
	if latest == nil {
		return result
	}
	for _, rule := range latest.Rules {
		result[rule.StableRuleId] = rule
	}
	return result
}

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
