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
)

type ScenarioIterationAppInterface interface {
	ListScenarioIterations(ctx context.Context, orgID string, filters app.GetScenarioIterationFilters) ([]app.ScenarioIteration, error)
	CreateScenarioIteration(ctx context.Context, orgID string, scenarioIteration app.CreateScenarioIterationInput) (app.ScenarioIteration, error)
	GetScenarioIteration(ctx context.Context, orgID string, scenarioIterationID string) (app.ScenarioIteration, error)
	UpdateScenarioIteration(ctx context.Context, organizationID string, rule app.UpdateScenarioIterationInput) (app.ScenarioIteration, error)
}

type APIScenarioIterationBody struct {
	TriggerCondition     json.RawMessage            `json:"triggerCondition,omitempty"`
	Rules                []APIScenarioIterationRule `json:"rules,omitempty"`
	ScoreReviewThreshold int                        `json:"scoreReviewThreshold"`
	ScoreRejectThreshold int                        `json:"scoreRejectThreshold"`
}

type APIScenarioIteration struct {
	ID         string    `json:"id"`
	ScenarioID string    `json:"scenarioId"`
	Version    int       `json:"version"`
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

		orgID, err := orgIDFromCtx(ctx)
		if err != nil {
			http.Error(w, "", http.StatusUnauthorized)
			return
		}

		input := ctx.Value(httpin.Input).(*ListScenarioIterationsInput)

		options := &utils.PtrToOptions{OmitZero: true}
		scenarioIterations, err := api.app.ListScenarioIterations(ctx, orgID, app.GetScenarioIterationFilters{
			ScenarioID: utils.PtrTo(input.ScenarioID, options),
		})
		if err != nil {
			// Could not execute request
			http.Error(w, fmt.Errorf("error getting scenario(id: %s) iterations: %w", input.ScenarioID, err).Error(), http.StatusInternalServerError)
			return
		}

		apiScenarioIterations := make([]APIScenarioIteration, len(scenarioIterations))
		for i, si := range scenarioIterations {
			apiScenarioIterations[i] = NewAPIScenarioIteration(si)
		}

		err = json.NewEncoder(w).Encode(apiScenarioIterations)
		if err != nil {
			// Could not encode JSON
			http.Error(w, fmt.Errorf("could not encode response JSON: %w", err).Error(), http.StatusInternalServerError)
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
	Body *CreateScenarioIterationBody `in:"body=json"`
}

func (api *API) CreateScenarioIteration() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		orgID, err := orgIDFromCtx(ctx)
		if err != nil {
			http.Error(w, "", http.StatusUnauthorized)
			return
		}

		input := ctx.Value(httpin.Input).(*CreateScenarioIterationInput)

		createScenarioIterationInput := app.CreateScenarioIterationInput{
			ScenarioID: input.Body.ScenarioID,
		}

		if input.Body.Body != nil {
			createScenarioIterationInput.Body = &app.CreateScenarioIterationBody{
				ScoreReviewThreshold: input.Body.Body.ScoreReviewThreshold,
				ScoreRejectThreshold: input.Body.Body.ScoreRejectThreshold,
				Rules:                make([]app.CreateRuleInput, len(input.Body.Body.Rules)),
			}

			for i, rule := range input.Body.Body.Rules {
				formula, err := operators.UnmarshalOperatorBool(rule.Formula)
				if err != nil {
					http.Error(w, fmt.Errorf("could not unmarshal formula: %w", err).Error(), http.StatusUnprocessableEntity)
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

			if input.Body.Body.TriggerCondition != nil {
				triggerCondition, err := operators.UnmarshalOperatorBool(*input.Body.Body.TriggerCondition)
				if err != nil {
					http.Error(w, fmt.Errorf("could not unmarshal trigger condition: %w", err).Error(), http.StatusUnprocessableEntity)
					return
				}
				createScenarioIterationInput.Body.TriggerCondition = triggerCondition
			}
		}

		si, err := api.app.CreateScenarioIteration(ctx, orgID, createScenarioIterationInput)
		if err != nil {
			// Could not execute request
			// TODO(errors): handle missing fields error ?
			http.Error(w, fmt.Errorf("error getting scenarios: %w", err).Error(), http.StatusInternalServerError)
			return
		}

		apiScenarioIterationWithBody, err := NewAPIScenarioIterationWithBody(si)
		if err != nil {
			http.Error(w, fmt.Errorf("could not create new api scenario iteration: %w", err).Error(), http.StatusInternalServerError)
			return
		}
		err = json.NewEncoder(w).Encode(apiScenarioIterationWithBody)
		if err != nil {
			http.Error(w, fmt.Errorf("could not encode response JSON: %w", err).Error(), http.StatusInternalServerError)
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

		orgID, err := orgIDFromCtx(ctx)
		if err != nil {
			http.Error(w, "", http.StatusUnauthorized)
			return
		}

		input := ctx.Value(httpin.Input).(*GetScenarioIterationInput)

		si, err := api.app.GetScenarioIteration(ctx, orgID, input.ScenarioIterationID)
		if err != nil {
			// Could not execute request
			http.Error(w, fmt.Errorf("error getting scenarioIterationID(id: %s): %w", input.ScenarioIterationID, err).Error(), http.StatusInternalServerError)
			return
		}

		apiScenarioIterationWithBody, err := NewAPIScenarioIterationWithBody(si)
		if err != nil {
			http.Error(w, fmt.Errorf("could not create new api scenario iteration: %w", err).Error(), http.StatusInternalServerError)
			return
		}
		err = json.NewEncoder(w).Encode(apiScenarioIterationWithBody)
		if err != nil {
			// Could not encode JSON
			http.Error(w, fmt.Errorf("could not encode response JSON: %w", err).Error(), http.StatusInternalServerError)
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
	Body                *UpdateScenarioIterationBody `in:"body=json"`
}

func (api *API) UpdateScenarioIteration() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		orgID, err := orgIDFromCtx(ctx)
		if err != nil {
			http.Error(w, "", http.StatusUnauthorized)
			return
		}

		input := ctx.Value(httpin.Input).(*UpdateScenarioIterationInput)

		if input.Body.Body == nil {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		updateScenarioIterationInput := app.UpdateScenarioIterationInput{
			ID: input.ScenarioIterationID,
			Body: &app.UpdateScenarioIterationBody{
				ScoreReviewThreshold: input.Body.Body.ScoreReviewThreshold,
				ScoreRejectThreshold: input.Body.Body.ScoreRejectThreshold,
			},
		}

		if input.Body.Body.TriggerCondition != nil {
			triggerCondition, err := operators.UnmarshalOperatorBool(*input.Body.Body.TriggerCondition)
			if err != nil {
				http.Error(w, fmt.Errorf("could not unmarshal triggerCondition: %w", err).Error(), http.StatusUnprocessableEntity)
				return
			}
			updateScenarioIterationInput.Body.TriggerCondition = triggerCondition
		}

		updatedSI, err := api.app.UpdateScenarioIteration(ctx, orgID, updateScenarioIterationInput)
		if errors.Is(err, app.ErrScenarioIterationNotDraft) {
			http.Error(w, err.Error(), http.StatusForbidden)
			return
		} else if err != nil {
			// Could not execute request
			// TODO(errors): handle missing fields error ?
			http.Error(w, fmt.Errorf("error updating scenario iteration: %w", err).Error(), http.StatusInternalServerError)
			return
		}

		apiRule, err := NewAPIScenarioIterationWithBody(updatedSI)
		if err != nil {
			http.Error(w, fmt.Errorf("could not create new api scenario iteration: %w", err).Error(), http.StatusInternalServerError)
			return
		}
		err = json.NewEncoder(w).Encode(apiRule)
		if err != nil {
			http.Error(w, fmt.Errorf("could not encode response JSON: %w", err).Error(), http.StatusInternalServerError)
			return
		}
	}
}
