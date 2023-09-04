package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"marble/marble-backend/dto"
	"marble/marble-backend/models"
	"marble/marble-backend/models/ast"
	"marble/marble-backend/utils"
	"net/http"

	"github.com/ggicci/httpin"
)

type ListScenarioIterationsInput struct {
	ScenarioId string `in:"query=scenarioId"`
}

func (api *API) ListScenarioIterations() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		input := r.Context().Value(httpin.Input).(*ListScenarioIterationsInput)

		usecase := api.UsecasesWithCreds(r).NewScenarioIterationUsecase()
		scenarioIterations, err := usecase.ListScenarioIterations(models.GetScenarioIterationFilters{
			ScenarioId: utils.PtrTo(input.ScenarioId, &utils.PtrToOptions{OmitZero: true}),
		})
		if presentError(w, r, err) {
			return
		}
		scenarioIterationsDtos := make([]dto.ScenarioIterationWithBodyDto, len(scenarioIterations))
		for i, si := range scenarioIterations {
			if dto, err := dto.AdaptScenarioIterationWithBodyDto(si); presentError(w, r, err) {
				return
			} else {
				scenarioIterationsDtos[i] = dto
			}
		}
		PresentModel(w, scenarioIterationsDtos)

	}
}

func (api *API) CreateScenarioIteration() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		organizationId, err := utils.OrgIDFromCtx(ctx, r)
		if presentError(w, r, err) {
			return
		}

		input := ctx.Value(httpin.Input).(*dto.CreateScenarioIterationInput)

		createScenarioIterationInput := models.CreateScenarioIterationInput{
			ScenarioId: input.Payload.ScenarioId,
		}

		if input.Payload.Body != nil {
			createScenarioIterationInput.Body = &models.CreateScenarioIterationBody{
				ScoreReviewThreshold: input.Payload.Body.ScoreReviewThreshold,
				ScoreRejectThreshold: input.Payload.Body.ScoreRejectThreshold,
				BatchTriggerSQL:      input.Payload.Body.BatchTriggerSQL,
				Schedule:             input.Payload.Body.Schedule,
				Rules:                make([]models.CreateRuleInput, len(input.Payload.Body.Rules)),
			}

			for i, rule := range input.Payload.Body.Rules {
				createScenarioIterationInput.Body.Rules[i], err = dto.AdaptCreateRuleInput(rule, organizationId)
				if presentError(w, r, err) {
					return
				}
			}

			if input.Payload.Body.TriggerConditionAstExpression != nil {
				trigger, err := dto.AdaptASTNode(*input.Payload.Body.TriggerConditionAstExpression)
				if err != nil {
					presentError(w, r, fmt.Errorf("invalid trigger: %w %w", err, models.BadParameterError))
					return
				}
				createScenarioIterationInput.Body.TriggerConditionAstExpression = &trigger
			}

		}

		usecase := api.UsecasesWithCreds(r).NewScenarioIterationUsecase()
		si, err := usecase.CreateScenarioIteration(ctx, organizationId, createScenarioIterationInput)
		if presentError(w, r, err) {
			return
		}

		apiScenarioIterationWithBody, err := dto.AdaptScenarioIterationWithBodyDto(si)
		if presentError(w, r, err) {
			return
		}
		err = json.NewEncoder(w).Encode(apiScenarioIterationWithBody)
		if presentError(w, r, err) {
			return
		}
	}
}

func (api *API) CreateDraftFromIteration() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		organizationId, err := utils.OrgIDFromCtx(ctx, r)
		if presentError(w, r, err) {
			return
		}

		input := ctx.Value(httpin.Input).(*dto.CreateDraftFromScenarioIterationInput)

		usecase := api.UsecasesWithCreds(r).NewScenarioIterationUsecase()
		si, err := usecase.CreateDraftFromScenarioIteration(ctx, organizationId, input.ScenarioIterationId)
		if presentError(w, r, err) {
			return
		}

		apiScenarioIterationWithBody, err := dto.AdaptScenarioIterationWithBodyDto(si)
		if presentError(w, r, err) {
			return
		}
		err = json.NewEncoder(w).Encode(apiScenarioIterationWithBody)
		if presentError(w, r, err) {
			return
		}
	}
}

func requiredIterationParam(r *http.Request) (string, error) {
	return requiredUuidUrlParam(r, "scenarioIterationId")
}

func (api *API) GetScenarioIteration() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		iterationId, err := requiredIterationParam(r)
		if presentError(w, r, err) {
			return
		}

		usecase := api.UsecasesWithCreds(r).NewScenarioIterationUsecase()
		si, err := usecase.GetScenarioIteration(iterationId)
		if presentError(w, r, err) {
			return
		}

		scenarioIterationDto, err := dto.AdaptScenarioIterationWithBodyDto(si)
		if presentError(w, r, err) {
			return
		}
		PresentModel(w, scenarioIterationDto)
	}
}

func (api *API) UpdateScenarioIteration() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		logger := utils.LoggerFromContext(ctx)

		organizationId, err := utils.OrgIDFromCtx(ctx, r)
		if presentError(w, r, err) {
			return
		}

		input := ctx.Value(httpin.Input).(*dto.UpdateScenarioIterationInput)
		logger = logger.With(slog.String("scenarioIterationId", input.ScenarioIterationId), slog.String("organizationId", organizationId))

		if input.Payload.Body == nil {
			PresentNothing(w)
			return
		}

		updateScenarioIterationInput := models.UpdateScenarioIterationInput{
			Id: input.ScenarioIterationId,
			Body: &models.UpdateScenarioIterationBody{
				ScoreReviewThreshold: input.Payload.Body.ScoreReviewThreshold,
				ScoreRejectThreshold: input.Payload.Body.ScoreRejectThreshold,
				Schedule:             input.Payload.Body.Schedule,
				BatchTriggerSQL:      input.Payload.Body.BatchTriggerSQL,
			},
		}

		if input.Payload.Body.TriggerConditionAstExpression != nil {
			trigger, err := dto.AdaptASTNode(*input.Payload.Body.TriggerConditionAstExpression)
			if err != nil {
				presentError(w, r, fmt.Errorf("invalid trigger: %w %w", err, models.BadParameterError))
				return
			}
			updateScenarioIterationInput.Body.TriggerConditionAstExpression = &trigger
		}

		usecase := api.UsecasesWithCreds(r).NewScenarioIterationUsecase()
		updatedSI, err := usecase.UpdateScenarioIteration(ctx, organizationId, updateScenarioIterationInput)
		if errors.Is(err, models.ErrScenarioIterationNotDraft) {
			logger.WarnContext(ctx, "Cannot update scenario iteration that is not in draft state: \n"+err.Error())
			http.Error(w, "", http.StatusForbidden)
			return
		}

		if presentError(w, r, err) {
			return
		}

		iteration, err := dto.AdaptScenarioIterationWithBodyDto(updatedSI)
		if presentError(w, r, err) {
			return
		}

		PresentModel(w, struct {
			Iteration dto.ScenarioIterationWithBodyDto `json:"iteration"`
		}{
			Iteration: iteration,
		})
	}
}

type PostScenarioValidationInput struct {
	Body *struct {
		TriggerOrRule *dto.NodeDto `json:"trigger_or_rule"`
		RuleId        *string      `json:"rule_id"`
	} `in:"body=json"`
}

func adaptTriggerAndRuleIdFromInput(input *PostScenarioValidationInput) (triggerOrRule *ast.Node, ruleId *string, err error) {

	// body is optional
	if input != nil && input.Body != nil && input.Body.TriggerOrRule != nil {
		ruleId = input.Body.RuleId
		node, err := dto.AdaptASTNode(*input.Body.TriggerOrRule)
		if err != nil {
			return triggerOrRule, ruleId, fmt.Errorf("invalid rule or trigger: %w %w", err, models.BadParameterError)
		}
		triggerOrRule = &node
	}

	return triggerOrRule, ruleId, err
}

func (api *API) ValidateScenarioIteration() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		iterationId, err := requiredIterationParam(r)
		if presentError(w, r, err) {
			return
		}

		input, _ := r.Context().Value(httpin.Input).(*PostScenarioValidationInput)

		triggerOrRuleToReplace, ruleIdToReplace, err := adaptTriggerAndRuleIdFromInput(input)
		if presentError(w, r, err) {
			return
		}

		usecase := api.UsecasesWithCreds(r).NewScenarioIterationUsecase()
		scenarioValidation, err := usecase.ValidateScenarioIteration(iterationId, triggerOrRuleToReplace, ruleIdToReplace)

		if presentError(w, r, err) {
			return
		}

		PresentModel(w, struct {
			ScenarioValidation dto.ScenarioValidationDto `json:"scenario_validation"`
		}{
			ScenarioValidation: dto.AdaptScenarioValidationDto(scenarioValidation),
		})
	}
}
