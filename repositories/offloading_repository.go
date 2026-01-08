package repositories

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

func (repo *MarbleDbRepository) GetOffloadedDecisionRuleKey(
	orgId uuid.UUID, decisionId, ruleId, outcome string, createdAt time.Time,
) string {
	if outcome == "" {
		outcome = "no_hit"
	}

	return fmt.Sprintf("offloading/decision_rules/%s/%s/%d/%d/%s/%s", outcome, orgId.String(),
		createdAt.Year(), createdAt.Month(), decisionId, ruleId)
}
