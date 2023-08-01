package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"marble/marble-backend/dto"
	"marble/marble-backend/models"
	"marble/marble-backend/models/ast"
	"marble/marble-backend/utils"
	"net/http"

	"github.com/ggicci/httpin"
	"golang.org/x/exp/slog"
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
		logger := api.logger.With(slog.String("scenarioId", input.Payload.ScenarioId), slog.String("organizationId", organizationId))

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

				var formulaAstExpression *ast.Node
				if rule.FormulaAstExpression != nil {
					f, err := dto.AdaptASTNode(*rule.FormulaAstExpression)
					if err != nil {
						presentError(w, r, fmt.Errorf("could not unmarshal formula ast expression: %w %w", err, models.BadParameterError))
					}
					formulaAstExpression = &f
				}

				createScenarioIterationInput.Body.Rules[i] = models.CreateRuleInput{
					DisplayOrder:         rule.DisplayOrder,
					Name:                 rule.Name,
					Description:          rule.Description,
					FormulaAstExpression: formulaAstExpression,
					ScoreModifier:        rule.ScoreModifier,
				}
			}

			if input.Payload.Body.TriggerConditionAstExpression != nil {
				trigger, err := dto.AdaptASTNode(*input.Payload.Body.TriggerConditionAstExpression)
				if err != nil {
					presentError(w, r, fmt.Errorf("could not unmarshal trigger condition ast expression: %w %w", err, models.BadParameterError))
				}
				createScenarioIterationInput.Body.TriggerConditionAstExpression = &trigger
			}

		}

		usecase := api.UsecasesWithCreds(r).NewScenarioIterationUsecase()
		si, err := usecase.CreateScenarioIteration(ctx, organizationId, createScenarioIterationInput)
		if err != nil {
			logger.ErrorCtx(ctx, "Error creating scenario iteration: \n"+err.Error())
			http.Error(w, "", http.StatusInternalServerError)
			return
		}

		apiScenarioIterationWithBody, err := dto.AdaptScenarioIterationWithBodyDto(si)
		if err != nil {
			logger.ErrorCtx(ctx, "Error marshalling scenario iteration: \n"+err.Error())
			http.Error(w, "", http.StatusInternalServerError)
			return
		}
		err = json.NewEncoder(w).Encode(apiScenarioIterationWithBody)
		if err != nil {
			logger.ErrorCtx(ctx, "Could not encode response JSON: \n"+err.Error())
			http.Error(w, "", http.StatusInternalServerError)
			return
		}
	}
}

type GetScenarioIterationInput struct {
	ScenarioIterationId string `in:"path=scenarioIterationId"`
}

func (api *API) GetScenarioIteration() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		input := r.Context().Value(httpin.Input).(*GetScenarioIterationInput)

		usecase := api.UsecasesWithCreds(r).NewScenarioIterationUsecase()
		si, err := usecase.GetScenarioIteration(input.ScenarioIterationId)
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

		organizationId, err := utils.OrgIDFromCtx(ctx, r)
		if presentError(w, r, err) {
			return
		}

		input := ctx.Value(httpin.Input).(*dto.UpdateScenarioIterationInput)
		logger := api.logger.With(slog.String("scenarioIterationId", input.ScenarioIterationId), slog.String("organizationId", organizationId))

		if input.Payload.Body == nil {
			http.Error(w, "", http.StatusNoContent)
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
				presentError(w, r, fmt.Errorf("could not unmarshal trigger condition ast expression: %w %w", err, models.BadParameterError))
			}
			updateScenarioIterationInput.Body.TriggerConditionAstExpression = &trigger
		}

		usecase := api.UsecasesWithCreds(r).NewScenarioIterationUsecase()
		updatedSI, err := usecase.UpdateScenarioIteration(ctx, organizationId, updateScenarioIterationInput)
		if errors.Is(err, models.ErrScenarioIterationNotDraft) {
			logger.WarnCtx(ctx, "Cannot update scenario iteration that is not in draft state: \n"+err.Error())
			http.Error(w, "", http.StatusForbidden)
			return
		} else if errors.Is(err, models.NotFoundInRepositoryError) {
			http.Error(w, "", http.StatusNotFound)
			return
		} else if err != nil {
			logger.ErrorCtx(ctx, "Error updating scenario iteration: \n"+err.Error())
			http.Error(w, "", http.StatusInternalServerError)
			return
		}

		apiRule, err := dto.AdaptScenarioIterationWithBodyDto(updatedSI)
		if err != nil {
			logger.ErrorCtx(ctx, "Error marshalling API scenario iteration: \n"+err.Error())
			http.Error(w, "", http.StatusInternalServerError)
			return
		}
		err = json.NewEncoder(w).Encode(apiRule)
		if err != nil {
			logger.ErrorCtx(ctx, "Could not encode response JSON: \n"+err.Error())
			http.Error(w, "", http.StatusInternalServerError)
			return
		}
	}
}
