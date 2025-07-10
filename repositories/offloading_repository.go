package repositories

import (
	"fmt"
	"time"
)

func (repo *MarbleDbRepository) GetOffloadedDecisionRuleKey(
	orgId, decisionId, ruleId, outcome string, createdAt time.Time,
) string {
	if outcome == "" {
		outcome = "no_hit"
	}

	return fmt.Sprintf("offloading/decision_rules/%s/%s/%d/%d/%s/%s", outcome, orgId,
		createdAt.Year(), createdAt.Month(), decisionId, ruleId)
}
