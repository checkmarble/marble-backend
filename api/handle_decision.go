package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"time"

	"marble/marble-backend/app"

	"github.com/go-chi/chi/v5"
)

type DecisionInterface interface {
	CreateDecision(ctx context.Context, organizationID string, scenarioID string, dynamicStructWithReader app.DynamicStructWithReader, payload app.Payload) (app.Decision, error)
	GetDecision(ctx context.Context, organizationID string, requestedDecisionID string) (app.Decision, error)
}

type APIError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type APIDecision struct {
	ID            string              `json:"id"`
	CreatedAt     time.Time           `json:"created_at"`
	TriggerObject map[string]any      `json:"trigger_object"`
	Outcome       string              `json:"outcome"`
	Scenario      APIDecisionScenario `json:"scenario"`
	Rules         []APIDecisionRule   `json:"rules"`
	Score         int                 `json:"score"`
	Error         *APIError           `json:"error"`
}

type APIDecisionScenario struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Version     int    `json:"version"`
}

func NewAPIDecision(decision app.Decision) APIDecision {
	apiDecision := APIDecision{
		ID:            decision.ID,
		CreatedAt:     decision.CreatedAt,
		TriggerObject: decision.Payload.Data,
		Outcome:       decision.Outcome.String(),
		Scenario: APIDecisionScenario{
			ID:          decision.ScenarioID,
			Name:        decision.ScenarioName,
			Description: decision.ScenarioDescription,
			Version:     decision.ScenarioVersion,
		},
		Score: decision.Score,
	}

	for _, ruleExecution := range decision.RuleExecutions {
		apiDecision.Rules = append(apiDecision.Rules, NewAPIDecisionRule(ruleExecution))
	}

	// Error added here to make sure it does not appear if empty
	// Otherwise, by default it will generate an empty APIError{}
	if int(decision.DecisionError) != 0 {
		apiDecision.Error = &APIError{int(decision.DecisionError), decision.DecisionError.String()}
	}

	return apiDecision
}

type APIDecisionRule struct {
	Name          string    `json:"name"`
	Description   string    `json:"description"`
	ScoreModifier int       `json:"score_modifier"`
	Result        bool      `json:"result"`
	Error         *APIError `json:"error"`
}

func NewAPIDecisionRule(rule app.RuleExecution) APIDecisionRule {
	apiDecisionRule := APIDecisionRule{
		Name:          rule.Rule.Name,
		Description:   rule.Rule.Description,
		ScoreModifier: rule.ResultScoreModifier,
		Result:        rule.Result,
	}

	// Error added here to make sure it does not appear if empty
	// Otherwise, by default it will generate an empty APIError{}
	if int(rule.Error) != 0 {
		apiDecisionRule.Error = &APIError{int(rule.Error), rule.Error.String()}
	}

	return apiDecisionRule
}

func (api *API) handleGetDecision() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		orgID, err := orgIDFromCtx(ctx)
		if err != nil {
			http.Error(w, "", http.StatusUnauthorized)
			return
		}
		decisionID := chi.URLParam(r, "decisionID")

		decision, err := api.app.GetDecision(ctx, orgID, decisionID)
		if errors.Is(err, app.ErrNotFoundInRepository) {
			http.Error(w, "", http.StatusNotFound)
			return
		} else if err != nil {
			// Could not execute request
			http.Error(w, fmt.Errorf("error getting decision: %w", err).Error(), http.StatusInternalServerError)
			return
		}

		err = json.NewEncoder(w).Encode(NewAPIDecision(decision))
		if err != nil {
			// Could not encode JSON
			http.Error(w, fmt.Errorf("could not encode response JSON: %w", err).Error(), http.StatusInternalServerError)
			return
		}
	}
}

type CreateDecisionInput struct {
	ScenarioID        string          `json:"scenario_id"`
	TriggerObjectRaw  json.RawMessage `json:"trigger_object"`
	TriggerObjectType string          `json:"object_type"`
}

func (api *API) handlePostDecision() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		orgID, err := orgIDFromCtx(ctx)
		if err != nil {
			http.Error(w, "", http.StatusUnauthorized)
			return
		}

		requestData := &CreateDecisionInput{}
		err = json.NewDecoder(r.Body).Decode(requestData)
		if err != nil {
			// Could not parse JSON
			http.Error(w, fmt.Errorf("could not parse input JSON: %w", err).Error(), http.StatusUnprocessableEntity)
			return
		}

		dataModel, err := api.app.GetDataModel(ctx, orgID)
		if err != nil {
			log.Printf("Unable to find datamodel by orgId for ingestion: %v", err)
			http.Error(w, "No data model found for this organization ID.", http.StatusInternalServerError) // 500
			return
		}

		tables := dataModel.Tables
		table, ok := tables[requestData.TriggerObjectType]
		if !ok {
			log.Printf("Table %s not found in data model for organization %s", requestData.TriggerObjectType, orgID)
			http.Error(w, "No data model found for this object type.", http.StatusNotFound) // 404
			return
		}

		payloadStructWithReaderPtr, err := app.ParseToDataModelObject(ctx, table, requestData.TriggerObjectRaw)
		if err != nil {
			if errors.Is(err, app.ErrFormatValidation) {
				http.Error(w, "Format validation error", http.StatusUnprocessableEntity) // 422
				return
			}
			log.Printf("Unexpected error while parsing to data model object: %v", err)
			http.Error(w, "", http.StatusInternalServerError) // 500
			return
		}

		// make a decision
		triggerObjectMap := make(map[string]interface{})
		err = json.Unmarshal(requestData.TriggerObjectRaw, &triggerObjectMap)
		if err != nil {
			http.Error(w, fmt.Errorf("could not unmarshal trigger object: %w", err).Error(), http.StatusUnprocessableEntity)
			return
		}
		payload := app.Payload{TableName: requestData.TriggerObjectType, Data: triggerObjectMap}
		decision, err := api.app.CreateDecision(ctx, orgID, requestData.ScenarioID, *payloadStructWithReaderPtr, payload)
		if errors.Is(err, app.ErrScenarioNotFound) {
			http.Error(w, "scenario not found", http.StatusNotFound)
			return
		} else if err != nil {
			// Could not execute request
			http.Error(w, fmt.Errorf("could not create a decision: %w", err).Error(), http.StatusInternalServerError)
			return
		}

		err = json.NewEncoder(w).Encode(NewAPIDecision(decision))
		if err != nil {
			// Could not encode JSON
			http.Error(w, fmt.Errorf("could not encode response JSON: %w", err).Error(), http.StatusInternalServerError)
			return
		}
	}
}
