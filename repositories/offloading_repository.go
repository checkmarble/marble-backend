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

func (repo *MarbleDbRepository) GetScoreComputationEvaluationKey(ruleset models.ScoringRuleset, score models.ScoringScore) string {
	return fmt.Sprintf("offloading/score_computations/%s/%s/%d/%d/%s", ruleset.OrgId.String(), ruleset.RecordType,
		score.CreatedAt.Year(), score.CreatedAt.Month(), score.Id.String())
}

// GetOffloadedScreeningMatchKey returns the deterministic blob key under which a screening
// match payload is offloaded. The key is fully derivable from data we already have, so we do
// not store it in the database: a populated payload column means the row is legacy (read it
// directly), an empty column means the payload lives at this key in blob storage.
func (repo *MarbleDbRepository) GetOffloadedScreeningMatchKey(orgId uuid.UUID, screeningId, matchId string) string {
	return fmt.Sprintf("screening/match/%s/%s/%s", orgId.String(), screeningId, matchId)
}

// GetOffloadedContinuousScreeningMatchKey returns the deterministic blob key for a continuous
// screening match payload.
func (repo *MarbleDbRepository) GetOffloadedContinuousScreeningMatchKey(orgId, continuousScreeningId, matchId uuid.UUID) string {
	return fmt.Sprintf("continuous_screening/match/%s/%s/%s", orgId.String(),
		continuousScreeningId.String(), matchId.String())
}

// GetOffloadedContinuousScreeningEntityKey returns the deterministic blob key for the
// OpenSanctions entity payload attached to a (dataset-triggered) continuous screening.
func (repo *MarbleDbRepository) GetOffloadedContinuousScreeningEntityKey(orgId, continuousScreeningId uuid.UUID) string {
	return fmt.Sprintf("continuous_screening/entity/%s/%s", orgId.String(), continuousScreeningId.String())
}

// GetOffloadedFreeformSearchResultKey returns the deterministic blob key under which a saved
// freeform search's result array is offloaded. A populated result column means the row is legacy
// (read it directly); an empty column means the payload lives at this key in blob storage.
func (repo *MarbleDbRepository) GetOffloadedFreeformSearchResultKey(orgId, searchId uuid.UUID) string {
	return fmt.Sprintf("freeform_search/result/%s/%s", orgId.String(), searchId.String())
}
