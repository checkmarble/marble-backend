package repositories

import (
	"fmt"
	"time"

	"github.com/checkmarble/marble-backend/models"
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

func (repo *MarbleDbRepository) GetOffloadedDecisionEvaluationKey(orgId uuid.UUID, decision models.Decision) string {
	return fmt.Sprintf("offloading/rule_evaluations/%s/%s/%d/%d/%s", decision.Outcome, orgId,
		decision.CreatedAt.Year(), decision.CreatedAt.Month(), decision.DecisionId.String())
}
