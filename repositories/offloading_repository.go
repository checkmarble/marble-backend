package repositories

import (
	"fmt"
	"time"

	"github.com/checkmarble/marble-backend/models"
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

func (repo *MarbleDbRepository) GetOffloadedDecisionEvaluationKey(orgId string, decision models.Decision) string {
	return fmt.Sprintf("offloading/rule_evaluations/%s/%s/%d/%d/%s", decision.Outcome, orgId,
		decision.CreatedAt.Year(), decision.CreatedAt.Month(), decision.DecisionId.String())
}
