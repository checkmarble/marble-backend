package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"

	"marble/marble-backend/app"
)

func (a *API) handleDecisionPost() http.HandlerFunc {

	///////////////////////////////
	// Request and Response types defined in scope
	///////////////////////////////

	// handleDecisionPostRequest is the request body
	type handleDecisionPostRequest struct {
		ScenarioID        string          `json:"scenario_id"`
		TriggerObjectRaw  json.RawMessage `json:"trigger_object"`
		TriggerObjectType string          `json:"object_type"`
	}

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

		// Create an empty instance of handleQuotesPostRequest
		requestData := &handleDecisionPostRequest{}

		// Create a JSON decoder
		// works as a stream
		d := json.NewDecoder(r.Body)

		// Decode the stream to the handleQuotesPostRequest instance
		err = d.Decode(requestData)
		if err != nil {
			// Could not parse JSON
			http.Error(w, fmt.Errorf("could not parse input JSON: %w", err).Error(), http.StatusUnprocessableEntity)
			return
		}

		dataModel, err := a.app.GetDataModel(ctx, orgID)
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
		payload := app.Payload{TableName: requestData.TriggerObjectType, Data: triggerObjectMap}
		decision, err := a.app.CreateDecision(ctx, orgID, requestData.ScenarioID, *payloadStructWithReaderPtr, payload)

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
