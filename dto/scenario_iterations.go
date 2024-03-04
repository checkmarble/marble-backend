package dto

import (
	"fmt"
	"time"

	"github.com/cockroachdb/errors"

	"github.com/checkmarble/marble-backend/models"
)

// Read DTO
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

type ScenarioIterationBodyDto struct {
	TriggerConditionAstExpression *NodeDto  `json:"trigger_condition_ast_expression"`
	Rules                         []RuleDto `json:"rules"`
	ScoreReviewThreshold          *int      `json:"scoreReviewThreshold"`
	ScoreRejectThreshold          *int      `json:"scoreRejectThreshold"`
	BatchTriggerSQL               string    `json:"batchTriggerSql"`
	Schedule                      string    `json:"schedule"`
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
			return ScenarioIterationWithBodyDto{},
				fmt.Errorf("could not create new api scenario iteration rule: %w", err)
		}
		body.Rules[i] = apiRule
	}

	if si.TriggerConditionAstExpression != nil {
		triggerDto, err := AdaptNodeDto(*si.TriggerConditionAstExpression)
		if err != nil {
			return ScenarioIterationWithBodyDto{},
				fmt.Errorf("unable to marshal trigger condition ast expression: %w", err)
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

// Update iteration DTO
type UpdateScenarioIterationBody struct {
	Body struct {
		TriggerConditionAstExpression *NodeDto `json:"trigger_condition_ast_expression"`
		ScoreReviewThreshold          *int     `json:"scoreReviewThreshold,omitempty"`
		ScoreRejectThreshold          *int     `json:"scoreRejectThreshold,omitempty"`
		Schedule                      *string  `json:"schedule"`
		BatchTriggerSQL               *string  `json:"batchTriggerSQL"`
	} `json:"body,omitempty"`
}

func AdaptUpdateScenarioIterationInput(input UpdateScenarioIterationBody, iterationId string) (models.UpdateScenarioIterationInput, error) {
	updateScenarioIterationInput := models.UpdateScenarioIterationInput{
		Id: iterationId,
		Body: models.UpdateScenarioIterationBody{
			ScoreReviewThreshold: input.Body.ScoreReviewThreshold,
			ScoreRejectThreshold: input.Body.ScoreRejectThreshold,
			Schedule:             input.Body.Schedule,
			BatchTriggerSQL:      input.Body.BatchTriggerSQL,
		},
	}

	if input.Body.TriggerConditionAstExpression != nil {
		trigger, err := AdaptASTNode(*input.Body.TriggerConditionAstExpression)
		if err != nil {
			return models.UpdateScenarioIterationInput{}, errors.Wrap(
				models.BadParameterError,
				"invalid trigger",
			)
		}
		updateScenarioIterationInput.Body.TriggerConditionAstExpression = &trigger
	}

	return updateScenarioIterationInput, nil
}

// Create iteration DTO
type CreateScenarioIterationBody struct {
	ScenarioId string `json:"scenarioId"`
	Body       *struct {
		TriggerConditionAstExpression *NodeDto              `json:"trigger_condition_ast_expression"`
		Rules                         []CreateRuleInputBody `json:"rules"`
		ScoreReviewThreshold          *int                  `json:"scoreReviewThreshold,omitempty"`
		ScoreRejectThreshold          *int                  `json:"scoreRejectThreshold,omitempty"`
		Schedule                      string                `json:"schedule"`
		BatchTriggerSQL               string                `json:"batchTriggerSQL"`
	} `json:"body,omitempty"`
}

func AdaptCreateScenarioIterationInput(input CreateScenarioIterationBody, organizationId string) (models.CreateScenarioIterationInput, error) {
	createScenarioIterationInput := models.CreateScenarioIterationInput{
		ScenarioId: input.ScenarioId,
	}

	if input.Body != nil {
		createScenarioIterationInput.Body = &models.CreateScenarioIterationBody{
			ScoreReviewThreshold: input.Body.ScoreReviewThreshold,
			ScoreRejectThreshold: input.Body.ScoreRejectThreshold,
			BatchTriggerSQL:      input.Body.BatchTriggerSQL,
			Schedule:             input.Body.Schedule,
			Rules:                make([]models.CreateRuleInput, len(input.Body.Rules)),
		}

		for i, rule := range input.Body.Rules {
			var err error
			createScenarioIterationInput.Body.Rules[i], err =
				AdaptCreateRuleInput(rule, organizationId)
			if err != nil {
				return models.CreateScenarioIterationInput{}, err
			}
		}

		if input.Body.TriggerConditionAstExpression != nil {
			trigger, err := AdaptASTNode(*input.Body.TriggerConditionAstExpression)
			if err != nil {
				return models.CreateScenarioIterationInput{},
					errors.Wrap(models.BadParameterError, "invalid trigger")
			}
			createScenarioIterationInput.Body.TriggerConditionAstExpression = &trigger
		}

	}
	return createScenarioIterationInput, nil
}
