package usecases

import "github.com/checkmarble/marble-backend/models"

// previousCommittedRulesByStableId returns, for the most recently committed
// iteration among the given ones (highest non-nil Version), its rules
// indexed by StableRuleId. Returns an empty map if none of the iterations
// is committed yet.
func previousCommittedRulesByStableId(iterations []models.ScenarioIteration) map[string]models.Rule {
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

// rulesNeedingAiDescriptionGeneration returns the ids of the rules, among
// currentRules, that need a new AI-generated description: either they have
// no matching rule (by StableRuleId) in the previous committed iteration, or
// their formula changed since then. Rules with an unchanged formula keep
// whatever AiDescription they already carry forward from that rule.
func rulesNeedingAiDescriptionGeneration(
	currentRules []models.Rule,
	previousRulesByStableId map[string]models.Rule,
) []string {
	var ruleIds []string
	for _, rule := range currentRules {
		previous, existed := previousRulesByStableId[rule.StableRuleId]
		if !existed || !rule.HasSameFormula(previous) {
			ruleIds = append(ruleIds, rule.Id)
		}
	}
	return ruleIds
}
