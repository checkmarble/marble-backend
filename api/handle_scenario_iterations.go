package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"marble/marble-backend/dto"
	"marble/marble-backend/models"
	"marble/marble-backend/models/ast"
	"marble/marble-backend/models/operators"
	"marble/marble-backend/utils"
	"net/http"
	"time"

	"github.com/ggicci/httpin"
	"golang.org/x/exp/slog"
)

type APIScenarioIterationBody struct {
	TriggerCondition              json.RawMessage            `json:"triggerCondition"`
	TriggerConditionAstExpression *dto.NodeDto               `json:"trigger_condition_ast_expression"`
	Rules                         []APIScenarioIterationRule `json:"rules"`
	ScoreReviewThreshold          *int                       `json:"scoreReviewThreshold"`
	ScoreRejectThreshold          *int                       `json:"scoreRejectThreshold"`
	BatchTriggerSQL               string                     `json:"batchTriggerSql"`
	Schedule                      string                     `json:"schedule"`
}

type APIScenarioIteration struct {
	ID         string    `json:"id"`
	ScenarioID string    `json:"scenarioId"`
	Version    *int      `json:"version"`
	CreatedAt  time.Time `json:"createdAt"`
	UpdatedAt  time.Time `json:"updatedAt"`
}

func NewAPIScenarioIteration(si models.ScenarioIteration) APIScenarioIteration {
	return APIScenarioIteration{
		ID:         si.ID,
		ScenarioID: si.ScenarioID,
		Version:    si.Version,
		CreatedAt:  si.CreatedAt,
		UpdatedAt:  si.UpdatedAt,
	}
}

type APIScenarioIterationWithBody struct {
	APIScenarioIteration
	Body APIScenarioIterationBody `json:"body"`
}

func NewAPIScenarioIterationWithBody(si models.ScenarioIteration) (APIScenarioIterationWithBody, error) {
	body := APIScenarioIterationBody{
		ScoreReviewThreshold: si.Body.ScoreReviewThreshold,
		ScoreRejectThreshold: si.Body.ScoreRejectThreshold,
		BatchTriggerSQL:      si.Body.BatchTriggerSQL,
		Schedule:             si.Body.Schedule,
		Rules:                make([]APIScenarioIterationRule, len(si.Body.Rules)),
	}
	for i, rule := range si.Body.Rules {
		apiRule, err := NewAPIScenarioIterationRule(rule)
		if err != nil {
			return APIScenarioIterationWithBody{}, fmt.Errorf("could not create new api scenario iteration rule: %w", err)
		}
		body.Rules[i] = apiRule
	}

	if si.Body.TriggerCondition != nil {
		triggerConditionBytes, err := si.Body.TriggerCondition.MarshalJSON()
		if err != nil {
			return APIScenarioIterationWithBody{}, fmt.Errorf("unable to marshal trigger condition: %w", err)
		}
		body.TriggerCondition = triggerConditionBytes
	}

	if si.Body.TriggerConditionAstExpression != nil {
		triggerDto, err := dto.AdaptNodeDto(*si.Body.TriggerConditionAstExpression)
		if err != nil {
			return APIScenarioIterationWithBody{}, fmt.Errorf("unable to marshal trigger condition ast expression: %w", err)
		}
		body.TriggerConditionAstExpression = &triggerDto
	}

	return APIScenarioIterationWithBody{
		APIScenarioIteration: NewAPIScenarioIteration(si),
		Body:                 body,
	}, nil
}

type ListScenarioIterationsInput struct {
	ScenarioID string `in:"query=scenarioId"`
}

func (api *API) ListScenarioIterations() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		orgID, err := utils.OrgIDFromCtx(ctx, r)
		if presentError(w, r, err) {
			return
		}

		input := ctx.Value(httpin.Input).(*ListScenarioIterationsInput)
		logger := api.logger.With(slog.String("scenarioId", input.ScenarioID), slog.String("orgId", orgID))

		options := &utils.PtrToOptions{OmitZero: true}
		usecase := api.usecases.NewScenarioIterationUsecase()
		scenarioIterations, err := usecase.ListScenarioIterations(models.GetScenarioIterationFilters{
			OrganizationId: orgID,
			ScenarioID:     utils.PtrTo(input.ScenarioID, options),
		})
		if err != nil {
			logger.ErrorCtx(ctx, "Error Listing scenario iterations: \n"+err.Error())
			http.Error(w, "", http.StatusInternalServerError)
			return
		}

		apiScenarioIterations := make([]APIScenarioIteration, len(scenarioIterations))
		for i, si := range scenarioIterations {
			apiScenarioIterations[i] = NewAPIScenarioIteration(si)
		}

		err = json.NewEncoder(w).Encode(apiScenarioIterations)
		if err != nil {
			logger.ErrorCtx(ctx, "Could not encode response JSON: \n"+err.Error())
			http.Error(w, "", http.StatusInternalServerError)
			return
		}
	}
}

func (api *API) CreateScenarioIteration() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		orgID, err := utils.OrgIDFromCtx(ctx, r)
		if presentError(w, r, err) {
			return
		}

		input := ctx.Value(httpin.Input).(*dto.CreateScenarioIterationInput)
		logger := api.logger.With(slog.String("scenarioId", input.Payload.ScenarioID), slog.String("orgId", orgID))

		createScenarioIterationInput := models.CreateScenarioIterationInput{
			ScenarioID: input.Payload.ScenarioID,
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
				formula, err := operators.UnmarshalOperatorBool(rule.Formula)
				if err != nil {
					presentError(w, r, fmt.Errorf("could not unmarshal formula: %w %w", err, models.BadParameterError))
				}

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
					Formula:              formula,
					FormulaAstExpression: formulaAstExpression,
					ScoreModifier:        rule.ScoreModifier,
				}
			}

			if input.Payload.Body.TriggerCondition != nil {
				triggerCondition, err := operators.UnmarshalOperatorBool(*input.Payload.Body.TriggerCondition)
				if err != nil {
					presentError(w, r, fmt.Errorf("could not unmarshal trigger condition: %w %w", err, models.BadParameterError))
					return
				}
				createScenarioIterationInput.Body.TriggerCondition = triggerCondition
			}

			if input.Payload.Body.TriggerConditionAstExpression != nil {
				trigger, err := dto.AdaptASTNode(*input.Payload.Body.TriggerConditionAstExpression)
				if err != nil {
					presentError(w, r, fmt.Errorf("could not unmarshal trigger condition ast expression: %w %w", err, models.BadParameterError))
				}
				createScenarioIterationInput.Body.TriggerConditionAstExpression = &trigger
			}

		}

		usecase := api.usecases.NewScenarioIterationUsecase()
		si, err := usecase.CreateScenarioIteration(ctx, orgID, createScenarioIterationInput)
		if err != nil {
			logger.ErrorCtx(ctx, "Error creating scenario iteration: \n"+err.Error())
			http.Error(w, "", http.StatusInternalServerError)
			return
		}

		apiScenarioIterationWithBody, err := NewAPIScenarioIterationWithBody(si)
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
	ScenarioIterationID string `in:"path=scenarioIterationID"`
}

func (api *API) GetScenarioIteration() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		orgID, err := utils.OrgIDFromCtx(ctx, r)
		if presentError(w, r, err) {
			return
		}

		input := ctx.Value(httpin.Input).(*GetScenarioIterationInput)
		logger := api.logger.With(slog.String("scenarioIterationId", input.ScenarioIterationID), slog.String("orgId", orgID))

		usecase := api.usecases.NewScenarioIterationUsecase()
		si, err := usecase.GetScenarioIteration(input.ScenarioIterationID)
		if errors.Is(err, models.NotFoundInRepositoryError) {
			http.Error(w, "", http.StatusNotFound)
			return
		} else if err != nil {
			logger.ErrorCtx(ctx, "Error getting scenario iteration: \n"+err.Error())
			http.Error(w, "", http.StatusInternalServerError)
			return
		}

		apiScenarioIterationWithBody, err := NewAPIScenarioIterationWithBody(si)
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

func (api *API) UpdateScenarioIteration() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		orgID, err := utils.OrgIDFromCtx(ctx, r)
		if presentError(w, r, err) {
			return
		}

		input := ctx.Value(httpin.Input).(*dto.UpdateScenarioIterationInput)
		logger := api.logger.With(slog.String("scenarioIterationId", input.ScenarioIterationID), slog.String("orgId", orgID))

		if input.Payload.Body == nil {
			http.Error(w, "", http.StatusNoContent)
			return
		}

		updateScenarioIterationInput := models.UpdateScenarioIterationInput{
			ID: input.ScenarioIterationID,
			Body: &models.UpdateScenarioIterationBody{
				ScoreReviewThreshold: input.Payload.Body.ScoreReviewThreshold,
				ScoreRejectThreshold: input.Payload.Body.ScoreRejectThreshold,
				Schedule:             input.Payload.Body.Schedule,
				BatchTriggerSQL:      input.Payload.Body.BatchTriggerSQL,
			},
		}

		if input.Payload.Body.TriggerCondition != nil {
			triggerCondition, err := operators.UnmarshalOperatorBool(*input.Payload.Body.TriggerCondition)
			if err != nil {
				logger.ErrorCtx(ctx, "Could not unmarshal trigger condition: \n"+err.Error())
				http.Error(w, "", http.StatusUnprocessableEntity)
				return
			}
			updateScenarioIterationInput.Body.TriggerCondition = triggerCondition
		}

		if input.Payload.Body.TriggerConditionAstExpression != nil {
			trigger, err := dto.AdaptASTNode(*input.Payload.Body.TriggerConditionAstExpression)
			if err != nil {
				presentError(w, r, fmt.Errorf("could not unmarshal trigger condition ast expression: %w %w", err, models.BadParameterError))
			}
			updateScenarioIterationInput.Body.TriggerConditionAstExpression = &trigger
		}

		usecase := api.usecases.NewScenarioIterationUsecase()
		updatedSI, err := usecase.UpdateScenarioIteration(ctx, orgID, updateScenarioIterationInput)
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

		apiRule, err := NewAPIScenarioIterationWithBody(updatedSI)
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
