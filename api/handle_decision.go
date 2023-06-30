package api

import (
	"encoding/json"
	"errors"
	"net/http"

	"marble/marble-backend/app"
	"marble/marble-backend/dto"
	"marble/marble-backend/models"
	"marble/marble-backend/utils"

	"github.com/ggicci/httpin"
	"golang.org/x/exp/slog"
)

type GetDecisionInput struct {
	DecisionID string `in:"path=decisionID"`
}

func (api *API) handleGetDecision() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		orgID, err := utils.OrgIDFromCtx(ctx, r)
		if presentError(w, r, err) {
			return
		}
		input := ctx.Value(httpin.Input).(*GetDecisionInput)
		decisionID := input.DecisionID

		usecase := api.usecases.NewDecisionUsecase()
		decision, err := usecase.GetDecision(utils.MustCredentialsFromCtx(ctx), orgID, decisionID)

		if presentError(w, r, err) {
			return
		}
		PresentModel(w, dto.NewAPIDecision(decision))
	}
}

func (api *API) handleListDecisions() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		orgID, err := utils.OrgIDFromCtx(ctx, r)
		if presentError(w, r, err) {
			return
		}
		logger := api.logger.With(slog.String("orgID", orgID))

		usecase := api.usecases.NewDecisionUsecase()
		decisions, err := usecase.ListDecisionsOfOrganization(orgID)
		if presentError(w, r, err) {
			return
		}
		apiDecisions := make([]dto.APIDecision, len(decisions))
		for i, decision := range decisions {
			apiDecisions[i] = dto.NewAPIDecision(decision)
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

type CreateDecisionInputDto struct {
	Body *CreateDecisionBody `in:"body=json"`
}

func (api *API) handlePostDecision() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		orgID, err := utils.OrgIDFromCtx(ctx, r)
		if presentError(w, r, err) {
			return
		}

		input := ctx.Value(httpin.Input).(*CreateDecisionInputDto)
		requestData := input.Body
		logger := api.logger.With(slog.String("scenarioId", requestData.ScenarioID), slog.String("objectType", requestData.TriggerObjectType), slog.String("orgId", orgID))

		organizationUsecase := api.usecases.NewOrganizationUseCase()
		dataModel, err := organizationUsecase.GetDataModel(ctx, orgID)
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

		payload, err := app.ParseToDataModelObject(table, requestData.TriggerObjectRaw)
		if errors.Is(err, models.FormatValidationError) {
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
		ClientObject := models.ClientObject{TableName: models.TableName(requestData.TriggerObjectType), Data: triggerObjectMap}
		decisionUsecase := api.usecases.NewDecisionUsecase()
		decision, err := decisionUsecase.CreateDecision(ctx, models.CreateDecisionInput{
			ScenarioID:              requestData.ScenarioID,
			ClientObject:            ClientObject,
			OrganizationID:          orgID,
			PayloadStructWithReader: payload,
		}, logger)
		if errors.Is(err, models.NotFoundError) || errors.Is(err, models.BadParameterError) {
			presentError(w, r, err)
			return
		} else if err != nil {
			logger.ErrorCtx(ctx, "Could not create a decision: \n"+err.Error())
			http.Error(w, "", http.StatusInternalServerError)
			return
		}

		err = json.NewEncoder(w).Encode(dto.NewAPIDecision(decision))
		if err != nil {
			logger.ErrorCtx(ctx, "error encoding response JSON: \n"+err.Error())
			http.Error(w, "", http.StatusInternalServerError)
			return
		}
	}
}
