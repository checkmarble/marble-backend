package dto

import (
	"fmt"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/google/uuid"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
)

// Read DTO
type ScenarioIterationWithBodyDto struct {
	ScenarioIterationDto
	Body ScenarioIterationBodyDto `json:"body"`
}

type ScenarioIterationDto struct {
	Id         string    `json:"id"`
	ScenarioId string    `json:"scenario_id"`
	Version    *int      `json:"version"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

func AdaptScenarioIterationMetadataDto(si models.ScenarioIterationMetadata) ScenarioIterationDto {
	return ScenarioIterationDto{
		Id:         si.Id,
		ScenarioId: si.ScenarioId,
		Version:    si.Version,
		CreatedAt:  si.CreatedAt,
		UpdatedAt:  si.UpdatedAt,
	}
}

type ScenarioIterationBodyDto struct {
	TriggerConditionAstExpression *NodeDto          `json:"trigger_condition_ast_expression"`
	Rules                         []RuleDto         `json:"rules"`
	SanctionCheckConfigs_deprec   []ScreeningConfig `json:"sanction_check_configs,omitempty"` //nolint:tagliatelle
	ScreeningConfigs              []ScreeningConfig `json:"screening_configs,omitempty"`
	ScoreReviewThreshold          *int              `json:"score_review_threshold"`
	ScoreBlockAndReviewThreshold  *int              `json:"score_block_and_review_threshold"`
	ScoreDeclineThreshold         *int              `json:"score_decline_threshold"`
	Schedule                      string            `json:"schedule"`
}

func AdaptScenarioIterationWithBodyDto(si models.ScenarioIteration) (ScenarioIterationWithBodyDto, error) {
	body := ScenarioIterationBodyDto{
		ScoreReviewThreshold:         si.ScoreReviewThreshold,
		ScoreBlockAndReviewThreshold: si.ScoreBlockAndReviewThreshold,
		ScoreDeclineThreshold:        si.ScoreDeclineThreshold,
		Schedule:                     si.Schedule,
		Rules:                        make([]RuleDto, len(si.Rules)),
	}
	for i, rule := range si.Rules {
		apiRule, err := AdaptRuleDto(rule)
		if err != nil {
			return ScenarioIterationWithBodyDto{},
				fmt.Errorf("could not create new api scenario iteration rule: %w", err)
		}
		body.Rules[i] = apiRule
	}

	if len(si.ScreeningConfigs) > 0 {
		sccs, err := pure_utils.MapErr(si.ScreeningConfigs, AdaptScreeningConfig)
		if err != nil {
			return ScenarioIterationWithBodyDto{},
				errors.Wrap(err, "could not parse the screening trigger rule")
		}

		body.ScreeningConfigs = sccs
		body.SanctionCheckConfigs_deprec = sccs
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
		ScoreReviewThreshold          *int     `json:"score_review_threshold,omitempty"`
		ScoreBlockAndReviewThreshold  *int     `json:"score_block_and_review_threshold,omitempty"`
		ScoreDeclineThreshold         *int     `json:"score_decline_threshold,omitempty"`
		Schedule                      *string  `json:"schedule"`
	} `json:"body,omitempty"`
}

func AdaptUpdateScenarioIterationInput(input UpdateScenarioIterationBody, iterationId string) (models.UpdateScenarioIterationInput, error) {
	updateScenarioIterationInput := models.UpdateScenarioIterationInput{
		Id: iterationId,
		Body: models.UpdateScenarioIterationBody{
			ScoreReviewThreshold:         input.Body.ScoreReviewThreshold,
			ScoreBlockAndReviewThreshold: input.Body.ScoreBlockAndReviewThreshold,
			ScoreDeclineThreshold:        input.Body.ScoreDeclineThreshold,
			Schedule:                     input.Body.Schedule,
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
	ScenarioId string `json:"scenario_id"`
	Body       struct {
		TriggerConditionAstExpression *NodeDto              `json:"trigger_condition_ast_expression"`
		Rules                         []CreateRuleInputBody `json:"rules"`
		ScoreReviewThreshold          *int                  `json:"score_review_threshold,omitempty"`
		ScoreBlockAndReviewThreshold  *int                  `json:"score_block_and_review_threshold,omitempty"`
		ScoreDeclineThreshold         *int                  `json:"score_decline_threshold,omitempty"`
		Schedule                      string                `json:"schedule"`
	} `json:"body"`
}

func AdaptCreateScenarioIterationInput(input CreateScenarioIterationBody, organizationId uuid.UUID) (models.CreateScenarioIterationInput, error) {
	createScenarioIterationInput := models.CreateScenarioIterationInput{
		ScenarioId: input.ScenarioId,
	}

	createScenarioIterationInput.Body = models.CreateScenarioIterationBody{
		ScoreReviewThreshold:         input.Body.ScoreReviewThreshold,
		ScoreBlockAndReviewThreshold: input.Body.ScoreBlockAndReviewThreshold,
		ScoreDeclineThreshold:        input.Body.ScoreDeclineThreshold,
		Schedule:                     input.Body.Schedule,
		Rules:                        make([]models.CreateRuleInput, len(input.Body.Rules)),
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

	return createScenarioIterationInput, nil
}
