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
	ScenarioId string    `json:"scenario_id"`
	Version    *int      `json:"version"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type ScenarioIterationBodyDto struct {
	TriggerConditionAstExpression *NodeDto             `json:"trigger_condition_ast_expression"`
	Rules                         []RuleDto            `json:"rules"`
	SanctionCheckConfig           *SanctionCheckConfig `json:"sanction_check_config,omitempty"`
	ScoreReviewThreshold          *int                 `json:"score_review_threshold"`
	ScoreBlockAndReviewThreshold  *int                 `json:"score_block_and_review_threshold"`
	ScoreRejectThreshold_deprec   *int                 `json:"score_reject_threshold"` //nolint:tagliatelle
	ScoreDeclineThreshold         *int                 `json:"score_decline_threshold"`
	Schedule                      string               `json:"schedule"`
}

func AdaptScenarioIterationWithBodyDto(si models.ScenarioIteration) (ScenarioIterationWithBodyDto, error) {
	body := ScenarioIterationBodyDto{
		ScoreReviewThreshold:         si.ScoreReviewThreshold,
		ScoreBlockAndReviewThreshold: si.ScoreBlockAndReviewThreshold,
		ScoreRejectThreshold_deprec:  si.ScoreDeclineThreshold,
		ScoreDeclineThreshold:        si.ScoreDeclineThreshold,
		Schedule:                     si.Schedule,
		Rules:                        make([]RuleDto, len(si.Rules)),
		SanctionCheckConfig:          nil,
	}
	for i, rule := range si.Rules {
		apiRule, err := AdaptRuleDto(rule)
		if err != nil {
			return ScenarioIterationWithBodyDto{},
				fmt.Errorf("could not create new api scenario iteration rule: %w", err)
		}
		body.Rules[i] = apiRule
	}
	if si.SanctionCheckConfig != nil {
		nodeDto, err := AdaptNodeDto(si.SanctionCheckConfig.TriggerRule)
		if err != nil {
			return ScenarioIterationWithBodyDto{},
				errors.Wrap(err, "could not parse the sanction check trigger rule")
		}
		queryDto, err := AdaptSanctionCheckConfigQuery(si.SanctionCheckConfig.Query)
		if err != nil {
			return ScenarioIterationWithBodyDto{},
				errors.Wrap(err, "could not parse the sanction check trigger rule")
		}

		body.SanctionCheckConfig = &SanctionCheckConfig{
			Datasets:      si.SanctionCheckConfig.Datasets,
			ForceOutcome:  nil,
			ScoreModifier: &si.SanctionCheckConfig.Outcome.ScoreModifier,
			TriggerRule:   &nodeDto,
			Query:         &queryDto,
		}

		if si.SanctionCheckConfig.Outcome.ForceOutcome != models.UnsetForcedOutcome {
			body.SanctionCheckConfig.ForceOutcome =
				si.SanctionCheckConfig.Outcome.ForceOutcome.MaybeString()
		}
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
		ScoreRejectThreshold_deprec   *int     `json:"score_reject_threshold,omitempty"` //nolint:tagliatelle
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

	if input.Body.ScoreDeclineThreshold == nil {
		updateScenarioIterationInput.Body.ScoreDeclineThreshold = input.Body.ScoreRejectThreshold_deprec
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
	Body       *struct {
		TriggerConditionAstExpression *NodeDto              `json:"trigger_condition_ast_expression"`
		Rules                         []CreateRuleInputBody `json:"rules"`
		ScoreReviewThreshold          *int                  `json:"score_review_threshold,omitempty"`
		ScoreBlockAndReviewThreshold  *int                  `json:"score_block_and_review_threshold,omitempty"`
		ScoreRejectThreshold_deprec   *int                  `json:"score_reject_threshold,omitempty"` //nolint:tagliatelle
		ScoreDeclineThreshold         *int                  `json:"score_decline_threshold,omitempty"`
		Schedule                      string                `json:"schedule"`
	} `json:"body,omitempty"`
}

func AdaptCreateScenarioIterationInput(input CreateScenarioIterationBody, organizationId string) (models.CreateScenarioIterationInput, error) {
	createScenarioIterationInput := models.CreateScenarioIterationInput{
		ScenarioId: input.ScenarioId,
	}

	if input.Body != nil {
		createScenarioIterationInput.Body = &models.CreateScenarioIterationBody{
			ScoreReviewThreshold:         input.Body.ScoreReviewThreshold,
			ScoreBlockAndReviewThreshold: input.Body.ScoreBlockAndReviewThreshold,
			ScoreDeclineThreshold:        input.Body.ScoreDeclineThreshold,
			Schedule:                     input.Body.Schedule,
			Rules:                        make([]models.CreateRuleInput, len(input.Body.Rules)),
		}

		if input.Body.ScoreDeclineThreshold == nil {
			createScenarioIterationInput.Body.ScoreDeclineThreshold = input.Body.ScoreRejectThreshold_deprec
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
