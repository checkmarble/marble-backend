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
	Id                string    `json:"id"`
	ScenarioId_deprec string    `json:"scenarioId"` //nolint:tagliatelle
	ScenarioId        string    `json:"scenario_id"`
	Version           *int      `json:"version"`
	CreatedAt_deprec  time.Time `json:"createdAt"` //nolint:tagliatelle
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt_deprec  time.Time `json:"updatedAt"` //nolint:tagliatelle
	UpdatedAt         time.Time `json:"updated_at"`
}

type ScenarioIterationBodyDto struct {
	TriggerConditionAstExpression *NodeDto  `json:"trigger_condition_ast_expression"`
	Rules                         []RuleDto `json:"rules"`
	ScoreReviewThreshold_deprec   *int      `json:"scoreReviewThreshold"` //nolint:tagliatelle
	ScoreReviewThreshold          *int      `json:"score_review_threshold"`
	ScoreRejectThreshold_deprec   *int      `json:"scoreRejectThreshold"` //nolint:tagliatelle
	ScoreRejectThreshold          *int      `json:"score_reject_threshold"`
	Schedule                      string    `json:"schedule"`
}

func AdaptScenarioIterationWithBodyDto(si models.ScenarioIteration) (ScenarioIterationWithBodyDto, error) {
	body := ScenarioIterationBodyDto{
		ScoreReviewThreshold_deprec: si.ScoreReviewThreshold,
		ScoreReviewThreshold:        si.ScoreReviewThreshold,
		ScoreRejectThreshold_deprec: si.ScoreRejectThreshold,
		ScoreRejectThreshold:        si.ScoreRejectThreshold,
		Schedule:                    si.Schedule,
		Rules:                       make([]RuleDto, len(si.Rules)),
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
			Id:                si.Id,
			ScenarioId_deprec: si.ScenarioId,
			ScenarioId:        si.ScenarioId,
			Version:           si.Version,
			CreatedAt_deprec:  si.CreatedAt,
			CreatedAt:         si.CreatedAt,
			UpdatedAt_deprec:  si.UpdatedAt,
			UpdatedAt:         si.UpdatedAt,
		},
		Body: body,
	}, nil
}

// Update iteration DTO
type UpdateScenarioIterationBody struct {
	Body struct {
		TriggerConditionAstExpression *NodeDto `json:"trigger_condition_ast_expression"`
		ScoreReviewThreshold_deprec   *int     `json:"scoreReviewThreshold,omitempty"` //nolint:tagliatelle
		ScoreReviewThreshold          *int     `json:"score_review_threshold,omitempty"`
		ScoreRejectThreshold_deprec   *int     `json:"scoreRejectThreshold,omitempty"` //nolint:tagliatelle
		ScoreRejectThreshold          *int     `json:"score_reject_threshold,omitempty"`
		Schedule                      *string  `json:"schedule"`
	} `json:"body,omitempty"`
}

func AdaptUpdateScenarioIterationInput(input UpdateScenarioIterationBody, iterationId string) (models.UpdateScenarioIterationInput, error) {
	updateScenarioIterationInput := models.UpdateScenarioIterationInput{
		Id: iterationId,
		Body: models.UpdateScenarioIterationBody{
			ScoreReviewThreshold: input.Body.ScoreReviewThreshold,
			ScoreRejectThreshold: input.Body.ScoreRejectThreshold,
			Schedule:             input.Body.Schedule,
		},
	}

	// TODO remove deprec
	if updateScenarioIterationInput.Body.ScoreReviewThreshold == nil {
		updateScenarioIterationInput.Body.ScoreReviewThreshold = input.Body.ScoreReviewThreshold_deprec
	}
	if updateScenarioIterationInput.Body.ScoreRejectThreshold == nil {
		updateScenarioIterationInput.Body.ScoreRejectThreshold = input.Body.ScoreRejectThreshold_deprec
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
	ScenarioId_deprec string `json:"scenarioId"` //nolint:tagliatelle
	ScenarioId        string `json:"scenario_id"`
	Body              *struct {
		TriggerConditionAstExpression *NodeDto              `json:"trigger_condition_ast_expression"`
		Rules                         []CreateRuleInputBody `json:"rules"`
		ScoreReviewThreshold_deprec   *int                  `json:"scoreReviewThreshold,omitempty"` //nolint:tagliatelle
		ScoreReviewThreshold          *int                  `json:"score_review_threshold,omitempty"`
		ScoreRejectThreshold_deprec   *int                  `json:"scoreRejectThreshold,omitempty"` //nolint:tagliatelle
		ScoreRejectThreshold          *int                  `json:"score_reject_threshold,omitempty"`
		Schedule                      string                `json:"schedule"`
	} `json:"body,omitempty"`
}

func AdaptCreateScenarioIterationInput(input CreateScenarioIterationBody, organizationId string) (models.CreateScenarioIterationInput, error) {
	createScenarioIterationInput := models.CreateScenarioIterationInput{
		ScenarioId: input.ScenarioId,
	}
	// TODO remove deprec
	if createScenarioIterationInput.ScenarioId == "" {
		createScenarioIterationInput.ScenarioId = input.ScenarioId_deprec
	}

	if input.Body != nil {
		createScenarioIterationInput.Body = &models.CreateScenarioIterationBody{
			ScoreReviewThreshold: input.Body.ScoreReviewThreshold,
			ScoreRejectThreshold: input.Body.ScoreRejectThreshold,
			Schedule:             input.Body.Schedule,
			Rules:                make([]models.CreateRuleInput, len(input.Body.Rules)),
		}

		// TODO remove deprec
		if createScenarioIterationInput.Body.ScoreReviewThreshold == nil {
			createScenarioIterationInput.Body.ScoreReviewThreshold = input.Body.ScoreReviewThreshold_deprec
		}
		if createScenarioIterationInput.Body.ScoreRejectThreshold == nil {
			createScenarioIterationInput.Body.ScoreRejectThreshold = input.Body.ScoreRejectThreshold_deprec
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
