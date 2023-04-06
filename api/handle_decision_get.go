package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"marble/marble-backend/app"

	"github.com/go-chi/chi/v5"
)

func (a *API) handleDecisionGet() http.HandlerFunc {

	///////////////////////////////
	// Request and Response types defined in scope
	///////////////////////////////

	// return is a decision

	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		///////////////////////////////
		// Authorize request
		///////////////////////////////
		orgID, err := orgIDFromCtx(ctx)
		if err != nil {
			http.Error(w, "", http.StatusUnauthorized)
			return
		}

		////////////////////////////////////////////////////////////
		// Decode request
		////////////////////////////////////////////////////////////

		// decisionID already matches a UUID pattern thanks to the router
		decisionID := chi.URLParam(r, "decisionID")

		////////////////////////////////////////////////////////////
		// Execute request
		////////////////////////////////////////////////////////////

		decision, err := a.app.GetDecision(ctx, orgID, decisionID)

		if errors.Is(err, app.ErrNotFoundInRepository) {
			http.Error(w, "", http.StatusNotFound)
			return
		} else if err != nil {
			// Could not execute request
			http.Error(w, fmt.Errorf("error getting decision: %w", err).Error(), http.StatusInternalServerError)
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
		// This is set by default by the http.Server when writing to w
		// w.WriteHeader(http.StatusOK)
		// return

	}

}
