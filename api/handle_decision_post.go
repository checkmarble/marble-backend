package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"marble/marble-backend/app"

	"github.com/davecgh/go-spew/spew"
)

func (a *API) handleDecisionPost() http.HandlerFunc {

	///////////////////////////////
	// Request and Response types defined in scope
	///////////////////////////////

	// handleDecisionPostRequest is the request body
	type handleDecisionPostRequest struct {
		ScenarioID    string         `json:"scenario_id"`
		TriggerObject map[string]any `json:"trigger_object"`
	}

	// return is a decision

	return func(w http.ResponseWriter, r *http.Request) {

		///////////////////////////////
		// Authorize request
		///////////////////////////////
		orgID, err := orgIDFromCtx(r.Context())
		if err != nil {
			http.Error(w, "", http.StatusUnauthorized)
			return
		}

		////////////////////////////////////////////////////////////
		// Decode request
		////////////////////////////////////////////////////////////

		// Create an empty instance of handleQuotesPostRequest
		requestData := &handleDecisionPostRequest{}

		// Create a JSON decoder
		// works as a stream
		d := json.NewDecoder(r.Body)

		// Decode the stream to the handleQuotesPostRequest instance
		err = d.Decode(requestData)
		if err != nil {
			// Could not parse JSON
			http.Error(w, fmt.Errorf("could not parse input JSON: %w", err).Error(), http.StatusBadRequest)
			return
		}

		spew.Dump(requestData)

		////////////////////////////////////////////////////////////
		// Execute request
		////////////////////////////////////////////////////////////

		// decode and validate payload
		payload, err := a.app.PayloadFromTriggerObject(orgID, requestData.TriggerObject)

		if errors.Is(err, app.ErrTriggerObjectAndDataModelMismatch) {
			http.Error(w, fmt.Errorf("could not process trigger_object: %w", err).Error(), http.StatusBadRequest)
			return
		} else if err != nil {
			http.Error(w, fmt.Errorf("could not process trigger_object: %w", err).Error(), http.StatusInternalServerError)
			return
		}

		// make a decision
		decision, err := a.app.CreateDecision(orgID, requestData.ScenarioID, payload)

		if errors.Is(err, app.ErrScenarioNotFound) {
			http.Error(w, "scenario not found", http.StatusNotFound)
			return
		} else if err != nil {
			// Could not execute request
			http.Error(w, fmt.Errorf("could not create a decision: %w", err).Error(), http.StatusInternalServerError)
			return
		}

		////////////////////////////////////////////////////////////
		// Prepare response
		////////////////////////////////////////////////////////////

		// Create an empty instance of handleQuotesPostRequest

		responseRules := make([]APIDecisionRule, len(decision.RuleExecutions))
		for i := 0; i < len(decision.RuleExecutions); i++ {
			responseRules[i] = APIDecisionRule{
				Name:          decision.RuleExecutions[i].Rule.Name,
				Description:   decision.RuleExecutions[i].Rule.Description,
				ScoreModifier: decision.RuleExecutions[i].ResultScoreModifier,
				Result:        decision.RuleExecutions[i].Result,
				// Error:         APIError{int(decision.RuleExecutions[i].Error), decision.RuleExecutions[i].Error.String()},
			}

			// Error added here to make sure it does not appear if empty
			// Otherwise, by default it will generate an empty APIError{}
			if int(decision.RuleExecutions[i].Error) != 0 {
				responseRules[i].Error = &APIError{int(decision.RuleExecutions[i].Error), decision.RuleExecutions[i].Error.String()}
			}

		}

		responseScenario := APIDecisionScenario{
			ID:          decision.ScenarioID,
			Name:        decision.ScenarioName,
			Description: decision.ScenarioDescription,
			Version:     decision.ScenarioVersion,
		}

		responseDecision := APIDecision{
			ID:             decision.ID,
			Created_at:     decision.Created_at.Unix(),
			Trigger_object: decision.Payload.Data,
			Outcome:        decision.Outcome.String(),
			Scenario:       responseScenario,
			Rules:          responseRules,
			Score:          decision.Score,
		}

		// Error added here to make sure it does not appear if empty
		// Otherwise, by default it will generate an empty APIError{}
		if int(decision.DecisionError) != 0 {
			responseDecision.Error = &APIError{int(decision.DecisionError), decision.DecisionError.String()}
		}

		// Create a JSON decoder
		// works as a stream
		e := json.NewEncoder(w)

		// Encode the handleQuotesPostRequest to the response writer
		err = e.Encode(&responseDecision)
		if err != nil {
			// Could not encode JSON
			http.Error(w, fmt.Errorf("could not encode response JSON: %w", err).Error(), http.StatusInternalServerError)
			return
		}

		// No errors detected

		// https://pkg.go.dev/net/http#ResponseWriter
		// If WriteHeader is not called explicitly, the first call to Write will trigger an implicit WriteHeader(http.StatusOK)
		return

	}

}
