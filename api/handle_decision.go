package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"marble/marble-backend/app"
	"marble/marble-backend/models"
	"marble/marble-backend/utils"

	"github.com/ggicci/httpin"
	"golang.org/x/exp/slog"
)

type DecisionInterface interface {
	CreateDecision(ctx context.Context, input app.CreateDecisionInput, logger *slog.Logger) (app.Decision, error)
	GetDecision(ctx context.Context, organizationID string, requestedDecisionID string) (app.Decision, error)
	ListDecisions(ctx context.Context, organizationID string) ([]app.Decision, error)
}

type APIError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type APIDecision struct {
	ID                string              `json:"id"`
	CreatedAt         time.Time           `json:"created_at"`
	TriggerObject     map[string]any      `json:"trigger_object"`
	TriggerObjectType string              `json:"trigger_object_type"`
	Outcome           string              `json:"outcome"`
	Scenario          APIDecisionScenario `json:"scenario"`
	Rules             []APIDecisionRule   `json:"rules"`
	Score             int                 `json:"score"`
	Error             *APIError           `json:"error"`
}

type APIDecisionScenario struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Version     int    `json:"version"`
}

func NewAPIDecision(decision app.Decision) APIDecision {
	apiDecision := APIDecision{
		ID:                decision.ID,
		CreatedAt:         decision.CreatedAt,
		TriggerObjectType: decision.PayloadForArchive.TableName,
		TriggerObject:     decision.PayloadForArchive.Data,
		Outcome:           decision.Outcome.String(),
		Scenario: APIDecisionScenario{
			ID:          decision.ScenarioID,
			Name:        decision.ScenarioName,
			Description: decision.ScenarioDescription,
			Version:     decision.ScenarioVersion,
		},
		Score: decision.Score,
		Rules: make([]APIDecisionRule, len(decision.RuleExecutions)),
	}

	for i, ruleExecution := range decision.RuleExecutions {
		apiDecision.Rules[i] = NewAPIDecisionRule(ruleExecution)
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

type GetDecisionInput struct {
	DecisionID string `in:"path=decisionID"`
}

func (api *API) handleGetDecision() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		orgID, err := utils.OrgIDFromCtx(ctx)
		if presentError(ctx, api.logger, w, err) {
			return
		}
		input := ctx.Value(httpin.Input).(*GetDecisionInput)
		decisionID := input.DecisionID

		logger := api.logger.With(slog.String("decisionID", decisionID))

		decision, err := api.app.GetDecision(ctx, orgID, decisionID)
		if errors.Is(err, app.ErrNotFoundInRepository) {
			http.Error(w, "", http.StatusNotFound)
			return
		} else if err != nil {
			logger.ErrorCtx(ctx, "error getting decision: \n"+err.Error())
			http.Error(w, "", http.StatusInternalServerError)
			return
		}

		err = json.NewEncoder(w).Encode(NewAPIDecision(decision))
		if err != nil {
			logger.ErrorCtx(ctx, "error encoding response JSON: \n"+err.Error())
			http.Error(w, "", http.StatusInternalServerError)
			return
		}
	}
}

func (api *API) handleListDecisions() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		orgID, err := utils.OrgIDFromCtx(ctx)
		if presentError(ctx, api.logger, w, err) {
			return
		}
		logger := api.logger.With(slog.String("orgID", orgID))

		decisions, err := api.app.ListDecisions(ctx, orgID)
		if err != nil {
			logger.ErrorCtx(ctx, "error listing decisions: \n"+err.Error())
			http.Error(w, "", http.StatusInternalServerError)
			return
		}
		apiDecisions := make([]APIDecision, len(decisions))
		for i, decision := range decisions {
			apiDecisions[i] = NewAPIDecision(decision)
		}

		err = json.NewEncoder(w).Encode(apiDecisions)
		if err != nil {
			logger.ErrorCtx(ctx, "error encoding response JSON: \n"+err.Error())
			http.Error(w, "", http.StatusInternalServerError)
			return
		}
	}
}

type CreateDecisionBody struct {
	ScenarioID        string          `json:"scenario_id"`
	TriggerObjectRaw  json.RawMessage `json:"trigger_object"`
	TriggerObjectType string          `json:"object_type"`
}

type CreateDecisionInput struct {
	Body *CreateDecisionBody `in:"body=json"`
}

func (api *API) handlePostDecision() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		orgID, err := utils.OrgIDFromCtx(ctx)
		if presentError(ctx, api.logger, w, err) {
			return
		}

		input := ctx.Value(httpin.Input).(*CreateDecisionInput)
		requestData := input.Body
		logger := api.logger.With(slog.String("scenarioId", requestData.ScenarioID), slog.String("objectType", requestData.TriggerObjectType), slog.String("orgId", orgID))

		dataModel, err := api.app.GetDataModel(ctx, orgID)
		if err != nil {
			logger.ErrorCtx(ctx, "Unable to find datamodel by orgId for ingestion: \n"+err.Error())
			http.Error(w, "", http.StatusInternalServerError)
			return
		}

		tables := dataModel.Tables
		table, ok := tables[models.TableName(requestData.TriggerObjectType)]
		if !ok {
			logger.ErrorCtx(ctx, "Table not found in data model for organization")
			http.Error(w, "", http.StatusNotFound)
			return
		}

		payloadStructWithReader, err := app.ParseToDataModelObject(table, requestData.TriggerObjectRaw)
		if errors.Is(err, app.ErrFormatValidation) {
			http.Error(w, "Format validation error", http.StatusUnprocessableEntity) // 422
			return
		} else if err != nil {
			logger.ErrorCtx(ctx, "Unexpected error while parsing to data model object:\n"+err.Error())
			http.Error(w, "", http.StatusInternalServerError)
			return
		}

		// make a decision
		triggerObjectMap := make(map[string]interface{})
		err = json.Unmarshal(requestData.TriggerObjectRaw, &triggerObjectMap)
		if err != nil {
			logger.ErrorCtx(ctx, "Could not unmarshal trigger object: \n"+err.Error())
			http.Error(w, "", http.StatusUnprocessableEntity)
			return
		}
		payloadForArchive := models.PayloadForArchive{TableName: requestData.TriggerObjectType, Data: triggerObjectMap}
		decision, err := api.app.CreateDecision(ctx, app.CreateDecisionInput{
			ScenarioID:              requestData.ScenarioID,
			PayloadForArchive:       payloadForArchive,
			OrganizationID:          orgID,
			PayloadStructWithReader: payloadStructWithReader,
		}, logger)
		if errors.Is(err, app.ErrScenarioNotFound) {
			http.Error(w, "scenario not found", http.StatusNotFound)
			return
		} else if err != nil {
			logger.ErrorCtx(ctx, "Could not create a decision: \n"+err.Error())
			http.Error(w, "", http.StatusInternalServerError)
			return
		}

		err = json.NewEncoder(w).Encode(NewAPIDecision(decision))
		if err != nil {
			logger.ErrorCtx(ctx, "error encoding response JSON: \n"+err.Error())
			http.Error(w, "", http.StatusInternalServerError)
			return
		}
	}
}
