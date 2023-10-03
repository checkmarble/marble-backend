package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/usecases/payload_parser"
	"github.com/checkmarble/marble-backend/utils"

	"github.com/ggicci/httpin"
)

const defaultDecisionslimit = 10000

func (api *API) handleGetDecision(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	input := ctx.Value(httpin.Input).(*dto.GetDecisionInput)
	decisionId := input.DecisionId

	usecase := api.UsecasesWithCreds(r).NewDecisionUsecase()
	decision, err := usecase.GetDecision(decisionId)
	if presentError(w, r, err) {
		return
	}
	PresentModel(w, dto.NewAPIDecision(decision))
}

func (api *API) handleListDecisions(w http.ResponseWriter, r *http.Request) {
	usecase := api.UsecasesWithCreds(r).NewDecisionUsecase()
	decisions, err := usecase.ListDecisions(defaultDecisionslimit)
	if presentError(w, r, err) {
		return
	}

	fmt.Println(decisions)
	PresentModel(w, utils.Map(decisions, dto.NewAPIDecision))
}

func (api *API) handlePostDecision(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := utils.LoggerFromContext(ctx)

	organizationId, err := utils.OrgIDFromCtx(ctx, r)
	if presentError(w, r, err) {
		return
	}

	input := ctx.Value(httpin.Input).(*dto.CreateDecisionInputDto)
	requestData := input.Body
	logger = logger.With(slog.String("scenarioId", requestData.ScenarioId), slog.String("objectType", requestData.TriggerObjectType), slog.String("organizationId", organizationId))

	dataModelUseCase := api.UsecasesWithCreds(r).NewDataModelUseCase()
	dataModel, err := dataModelUseCase.GetDataModel(organizationId)
	if err != nil {
		logger.ErrorContext(ctx, "Unable to find datamodel by organizationId for ingestion: \n"+err.Error())
		http.Error(w, "", http.StatusInternalServerError)
		return
	}

	tables := dataModel.Tables
	table, ok := tables[models.TableName(requestData.TriggerObjectType)]
	if !ok {
		logger.ErrorContext(ctx, "Table not found in data model for organization")
		http.Error(w, "", http.StatusNotFound)
		return
	}

	payload, err := payload_parser.ParseToDataModelObject(table, requestData.TriggerObjectRaw)
	if errors.Is(err, models.FormatValidationError) {
		http.Error(w, "Format validation error", http.StatusUnprocessableEntity) // 422
		return
	} else if err != nil {
		logger.ErrorContext(ctx, "Unexpected error while parsing to data model object:\n"+err.Error())
		http.Error(w, "", http.StatusInternalServerError)
		return
	}

	// make a decision
	triggerObjectMap := make(map[string]interface{})
	err = json.Unmarshal(requestData.TriggerObjectRaw, &triggerObjectMap)
	if err != nil {
		logger.ErrorContext(ctx, "Could not unmarshal trigger object: \n"+err.Error())
		http.Error(w, "", http.StatusUnprocessableEntity)
		return
	}
	ClientObject := models.ClientObject{TableName: models.TableName(requestData.TriggerObjectType), Data: triggerObjectMap}
	decisionUsecase := api.UsecasesWithCreds(r).NewDecisionUsecase()

	decision, err := decisionUsecase.CreateDecision(ctx, models.CreateDecisionInput{
		ScenarioId:              requestData.ScenarioId,
		ClientObject:            ClientObject,
		OrganizationId:          organizationId,
		PayloadStructWithReader: payload,
	}, logger)
	if errors.Is(err, models.NotFoundError) || errors.Is(err, models.BadParameterError) {
		presentError(w, r, err)
		return
	} else if err != nil {
		logger.ErrorContext(ctx, "Could not create a decision: \n"+err.Error())
		http.Error(w, "", http.StatusInternalServerError)
		return
	}

	err = json.NewEncoder(w).Encode(dto.NewAPIDecision(decision))
	if err != nil {
		logger.ErrorContext(ctx, "error encoding response JSON: \n"+err.Error())
		http.Error(w, "", http.StatusInternalServerError)
		return
	}
}
