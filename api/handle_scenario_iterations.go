package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"marble/marble-backend/app"
	"marble/marble-backend/app/operators"
	"marble/marble-backend/utils"
	"net/http"
	"time"

	"github.com/ggicci/httpin"
	"golang.org/x/exp/slog"
)

type ScenarioIterationAppInterface interface {
	ListScenarioIterations(ctx context.Context, orgID string, filters app.GetScenarioIterationFilters) ([]app.ScenarioIteration, error)
	CreateScenarioIteration(ctx context.Context, orgID string, scenarioIteration app.CreateScenarioIterationInput) (app.ScenarioIteration, error)
	GetScenarioIteration(ctx context.Context, orgID string, scenarioIterationID string) (app.ScenarioIteration, error)
	UpdateScenarioIteration(ctx context.Context, organizationID string, rule app.UpdateScenarioIterationInput) (app.ScenarioIteration, error)
}

type APIScenarioIterationBody struct {
	TriggerCondition     json.RawMessage            `json:"triggerCondition"`
	Rules                []APIScenarioIterationRule `json:"rules"`
	ScoreReviewThreshold *int                       `json:"scoreReviewThreshold"`
	ScoreRejectThreshold *int                       `json:"scoreRejectThreshold"`
}

type APIScenarioIteration struct {
	ID         string    `json:"id"`
	ScenarioID string    `json:"scenarioId"`
	Version    *int      `json:"version"`
	CreatedAt  time.Time `json:"createdAt"`
	UpdatedAt  time.Time `json:"updatedAt"`
}

func NewAPIScenarioIteration(si app.ScenarioIteration) APIScenarioIteration {
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

func NewAPIScenarioIterationWithBody(si app.ScenarioIteration) (APIScenarioIterationWithBody, error) {
	body := APIScenarioIterationBody{
		ScoreReviewThreshold: si.Body.ScoreReviewThreshold,
		ScoreRejectThreshold: si.Body.ScoreRejectThreshold,
		Rules:                make([]APIScenarioIterationRule, len(si.Body.Rules)),
	}
	for i, rule := range si.Body.Rules {
		apiRule, err := NewAPIScenarioIterationRule(rule)
		if err != nil {
			return APIScenarioIterationWithBody{}, fmt.Errorf("Could not create new api scenario iteration rule: %w", err)
		}
		body.Rules[i] = apiRule
	}

	if si.Body.TriggerCondition != nil {
		triggerConditionBytes, err := si.Body.TriggerCondition.MarshalJSON()
		if err != nil {
			return APIScenarioIterationWithBody{}, fmt.Errorf("Unable to marshal trigger condition: %w", err)
		}
		body.TriggerCondition = triggerConditionBytes
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

		orgID, err := utils.OrgIDFromCtx(ctx)
		if presentError(ctx, api.logger, w, err) {
			return
		}

		input := ctx.Value(httpin.Input).(*ListScenarioIterationsInput)
		logger := api.logger.With(slog.String("scenarioId", input.ScenarioID), slog.String("orgId", orgID))

		options := &utils.PtrToOptions{OmitZero: true}
		scenarioIterations, err := api.app.ListScenarioIterations(ctx, orgID, app.GetScenarioIterationFilters{
			ScenarioID: utils.PtrTo(input.ScenarioID, options),
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

type CreateScenarioIterationBody struct {
	ScenarioID string `json:"scenarioId"`
	Body       *struct {
		TriggerCondition     *json.RawMessage                       `json:"triggerCondition,omitempty"`
		Rules                []CreateScenarioIterationRuleInputBody `json:"rules"`
		ScoreReviewThreshold *int                                   `json:"scoreReviewThreshold,omitempty"`
		ScoreRejectThreshold *int                                   `json:"scoreRejectThreshold,omitempty"`
	} `json:"body,omitempty"`
}

type CreateScenarioIterationInput struct {
	Payload *CreateScenarioIterationBody `in:"body=json"`
}

func (api *API) CreateScenarioIteration() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		orgID, err := utils.OrgIDFromCtx(ctx)
		if presentError(ctx, api.logger, w, err) {
			return
		}

		input := ctx.Value(httpin.Input).(*CreateScenarioIterationInput)
		logger := api.logger.With(slog.String("scenarioId", input.Payload.ScenarioID), slog.String("orgId", orgID))

		createScenarioIterationInput := app.CreateScenarioIterationInput{
			ScenarioID: input.Payload.ScenarioID,
		}

		if input.Payload.Body != nil {
			createScenarioIterationInput.Body = &app.CreateScenarioIterationBody{
				ScoreReviewThreshold: input.Payload.Body.ScoreReviewThreshold,
				ScoreRejectThreshold: input.Payload.Body.ScoreRejectThreshold,
				Rules:                make([]app.CreateRuleInput, len(input.Payload.Body.Rules)),
			}

			for i, rule := range input.Payload.Body.Rules {
				formula, err := operators.UnmarshalOperatorBool(rule.Formula)
				if err != nil {
					logger.ErrorCtx(ctx, "Could not unmarshal formula: \n"+err.Error())
					http.Error(w, "", http.StatusUnprocessableEntity)
					return
				}
				createScenarioIterationInput.Body.Rules[i] = app.CreateRuleInput{
					DisplayOrder:  rule.DisplayOrder,
					Name:          rule.Name,
					Description:   rule.Description,
					Formula:       formula,
					ScoreModifier: rule.ScoreModifier,
				}
			}

			if input.Payload.Body.TriggerCondition != nil {
				triggerCondition, err := operators.UnmarshalOperatorBool(*input.Payload.Body.TriggerCondition)
				if err != nil {
					logger.ErrorCtx(ctx, "Could not unmarshal trigger condition: \n"+err.Error())
					http.Error(w, "", http.StatusUnprocessableEntity)
					return
				}
				createScenarioIterationInput.Body.TriggerCondition = triggerCondition
			}
		}

		si, err := api.app.CreateScenarioIteration(ctx, orgID, createScenarioIterationInput)
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

		orgID, err := utils.OrgIDFromCtx(ctx)
		if presentError(ctx, api.logger, w, err) {
			return
		}

		input := ctx.Value(httpin.Input).(*GetScenarioIterationInput)
		logger := api.logger.With(slog.String("scenarioIterationId", input.ScenarioIterationID), slog.String("orgId", orgID))

		si, err := api.app.GetScenarioIteration(ctx, orgID, input.ScenarioIterationID)
		if errors.Is(err, app.ErrNotFoundInRepository) {
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

type UpdateScenarioIterationBody struct {
	Body *struct {
		TriggerCondition     *json.RawMessage `json:"triggerCondition,omitempty"`
		ScoreReviewThreshold *int             `json:"scoreReviewThreshold,omitempty"`
		ScoreRejectThreshold *int             `json:"scoreRejectThreshold,omitempty"`
	} `json:"body,omtiempty"`
}

type UpdateScenarioIterationInput struct {
	ScenarioIterationID string                       `in:"path=scenarioIterationID"`
	Payload             *UpdateScenarioIterationBody `in:"body=json"`
}

func (api *API) UpdateScenarioIteration() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		orgID, err := utils.OrgIDFromCtx(ctx)
		if presentError(ctx, api.logger, w, err) {
			return
		}

		input := ctx.Value(httpin.Input).(*UpdateScenarioIterationInput)
		logger := api.logger.With(slog.String("scenarioIterationId", input.ScenarioIterationID), slog.String("orgId", orgID))

		if input.Payload.Body == nil {
			http.Error(w, "", http.StatusNoContent)
			return
		}

		appUpdateScenarioIterationInput := app.UpdateScenarioIterationInput{
			ID: input.ScenarioIterationID,
			Body: &app.UpdateScenarioIterationBody{
				ScoreReviewThreshold: input.Payload.Body.ScoreReviewThreshold,
				ScoreRejectThreshold: input.Payload.Body.ScoreRejectThreshold,
			},
		}

		if input.Payload.Body.TriggerCondition != nil {
			triggerCondition, err := operators.UnmarshalOperatorBool(*input.Payload.Body.TriggerCondition)
			if err != nil {
				logger.ErrorCtx(ctx, "Could not unmarshal trigger condition: \n"+err.Error())
				http.Error(w, "", http.StatusUnprocessableEntity)
				return
			}
			appUpdateScenarioIterationInput.Body.TriggerCondition = triggerCondition
		}

		updatedSI, err := api.app.UpdateScenarioIteration(ctx, orgID, appUpdateScenarioIterationInput)
		if errors.Is(err, app.ErrScenarioIterationNotDraft) {
			logger.WarnCtx(ctx, "Cannot update scenario iteration that is not in draft state: \n"+err.Error())
			http.Error(w, "", http.StatusForbidden)
			return
		} else if errors.Is(err, app.ErrNotFoundInRepository) {
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
