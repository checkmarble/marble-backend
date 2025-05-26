package dto

import (
	"fmt"
	"time"

	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/utils"
)

type Decision struct {
	Id               string           `json:"id"`
	BatchExecutionId *string          `json:"batch_execution_id,omitempty"`
	Case             *Case            `json:"case,omitempty"`
	CreatedAt        time.Time        `json:"created_at"`
	TriggerObject    map[string]any   `json:"trigger_object"`
	Outcome          string           `json:"outcome"`
	ReviewStatus     *string          `json:"review_status"`
	Scenario         DecisionScenario `json:"scenario"`
	Score            int              `json:"score"`
	Rules            []DecisionRule   `json:"rules,omitzero"`
	Screenings       []SanctionCheck  `json:"screenings,omitzero"`
}

type DecisionScenario struct {
	Id          string `json:"id"`
	IterationId string `json:"iteration_id"`
	Version     string `json:"version"`
}

type DecisionRule struct {
	Id            string             `json:"id"`
	Name          string             `json:"name"`
	Outcome       string             `json:"outcome"`
	ScoreModifier int                `json:"score_modifier"`
	Error         *DecisionRuleError `json:"error,omitempty"`
}

type DecisionRuleError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func AdaptDecision(includeRules bool, ruleExecutions []models.RuleExecution, sanctionCheck *models.SanctionCheckWithMatches) func(models.Decision) Decision {
	return func(model models.Decision) Decision {
		d := Decision{
			Id:               model.DecisionId,
			CreatedAt:        model.CreatedAt,
			TriggerObject:    model.ClientObject.Data,
			Outcome:          model.Outcome.String(),
			ReviewStatus:     model.ReviewStatus,
			Score:            model.Score,
			BatchExecutionId: model.ScheduledExecutionId,
			Scenario: DecisionScenario{
				Id:          model.ScenarioId,
				IterationId: model.ScenarioIterationId,
				Version:     fmt.Sprintf("%d", model.ScenarioVersion),
			},
		}

		if model.Case != nil {
			d.Case = utils.Ptr(AdaptCase(*model.Case))
		}

		if includeRules {
			d.Rules = []DecisionRule{}
			d.Screenings = []SanctionCheck{}

			if ruleExecutions != nil {
				d.Rules = pure_utils.Map(ruleExecutions, AdaptDecisionRule)
			}

			if sanctionCheck != nil {
				d.Screenings = []SanctionCheck{AdaptSanctionCheck(false)(*sanctionCheck)}
			}
		}

		return d
	}
}

func AdaptDecisionRule(rule models.RuleExecution) DecisionRule {
	var ruleError *DecisionRuleError

	if rule.ExecutionError.String() != "" {
		ruleError = &DecisionRuleError{
			Code:    int(rule.ExecutionError),
			Message: rule.ExecutionError.String(),
		}
	}

	out := DecisionRule{
		Id:            rule.Rule.Id,
		Name:          rule.Rule.Name,
		Outcome:       rule.Outcome,
		ScoreModifier: rule.ResultScoreModifier,
		Error:         ruleError,
	}

	return out
}

func AdaptDecisionsMetadata(stats dto.DecisionsAggregateMetadata) map[string]any {
	return map[string]any{
		"total":            stats.Count.Total,
		"approve":          stats.Count.Approve,
		"review":           stats.Count.Review,
		"block_and_review": stats.Count.BlockAndReview,
		"decline":          stats.Count.Decline,
		"skipped":          stats.Count.Skipped,
	}
}
