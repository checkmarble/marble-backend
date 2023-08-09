package dto

import (
	"fmt"
	"marble/marble-backend/models"
	"time"
)

type ScenarioIterationBodyDto struct {
	TriggerConditionAstExpression *NodeDto  `json:"trigger_condition_ast_expression"`
	Rules                         []RuleDto `json:"rules"`
	ScoreReviewThreshold          *int      `json:"scoreReviewThreshold"`
	ScoreRejectThreshold          *int      `json:"scoreRejectThreshold"`
	BatchTriggerSQL               string    `json:"batchTriggerSql"`
	Schedule                      string    `json:"schedule"`
}

type ScenarioIterationWithBodyDto struct {
	ScenarioIterationDto
	Body ScenarioIterationBodyDto `json:"body"`
}

type ScenarioIterationDto struct {
	Id         string    `json:"id"`
	ScenarioId string    `json:"scenarioId"`
	Version    *int      `json:"version"`
	CreatedAt  time.Time `json:"createdAt"`
	UpdatedAt  time.Time `json:"updatedAt"`
}

func AdaptScenarioIterationWithBodyDto(si models.ScenarioIteration) (ScenarioIterationWithBodyDto, error) {
	body := ScenarioIterationBodyDto{
		ScoreReviewThreshold: si.ScoreReviewThreshold,
		ScoreRejectThreshold: si.ScoreRejectThreshold,
		BatchTriggerSQL:      si.BatchTriggerSQL,
		Schedule:             si.Schedule,
		Rules:                make([]RuleDto, len(si.Rules)),
	}
	for i, rule := range si.Rules {
		apiRule, err := AdaptRuleDto(rule)
		if err != nil {
			return ScenarioIterationWithBodyDto{}, fmt.Errorf("could not create new api scenario iteration rule: %w", err)
		}
		body.Rules[i] = apiRule
	}

	if si.TriggerConditionAstExpression != nil {
		triggerDto, err := AdaptNodeDto(*si.TriggerConditionAstExpression)
		if err != nil {
			return ScenarioIterationWithBodyDto{}, fmt.Errorf("unable to marshal trigger condition ast expression: %w", err)
		}
		body.TriggerConditionAstExpression = &triggerDto
	}

	return ScenarioIterationWithBodyDto{
		ScenarioIterationDto: ScenarioIterationDto{
			Id:         si.Id,
			ScenarioId: si.ScenarioId,
			Version:    si.Version,
			CreatedAt:  si.CreatedAt,
			UpdatedAt:  si.UpdatedAt,
		},
		Body: body,
	}, nil
}
