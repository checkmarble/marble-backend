package api

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/usecases/payload_parser"
	"github.com/checkmarble/marble-backend/utils"
)

const defaultDecisionsLimit = 10000

func (api *API) handleGetDecision(c *gin.Context) {
	decisionID := c.Param("decision_id")

	usecase := api.UsecasesWithCreds(c.Request).NewDecisionUsecase()
	decision, err := usecase.GetDecision(decisionID)
	if presentError(c.Writer, c.Request, err) {
		return
	}
	c.JSON(http.StatusOK, dto.NewAPIDecision(decision))
}

func (api *API) handleListDecisions(c *gin.Context) {
	organizationId, err := utils.OrgIDFromCtx(c.Request.Context(), c.Request)
	if presentError(c.Writer, c.Request, err) {
		return
	}

	var filters dto.DecisionFilters
	if err := c.ShouldBind(&filters); err != nil {
		c.Status(http.StatusBadRequest)
		return
	}

	usecase := api.UsecasesWithCreds(c.Request).NewDecisionUsecase()
	decisions, err := usecase.ListDecisions(organizationId, filters, defaultDecisionsLimit)
	if presentError(c.Writer, c.Request, err) {
		return
	}
	c.JSON(http.StatusOK, utils.Map(decisions, dto.NewAPIDecision))
}

func (api *API) handlePostDecision(c *gin.Context) {
	logger := utils.LoggerFromContext(c.Request.Context())

	organizationId, err := utils.OrgIDFromCtx(c.Request.Context(), c.Request)
	if presentError(c.Writer, c.Request, err) {
		return
	}

	var requestData dto.CreateDecisionBody
	if err := c.ShouldBindJSON(&requestData); err != nil {
		c.Status(http.StatusBadRequest)
		return
	}

	dataModelUseCase := api.UsecasesWithCreds(c.Request).NewDataModelUseCase()
	dataModel, err := dataModelUseCase.GetDataModel(organizationId)
	if err != nil {
		http.Error(c.Writer, "", http.StatusInternalServerError)
		return
	}

	tables := dataModel.Tables
	table, ok := tables[models.TableName(requestData.TriggerObjectType)]
	if !ok {
		http.Error(c.Writer, "", http.StatusNotFound)
		return
	}

	payload, err := payload_parser.ParseToDataModelObject(table, requestData.TriggerObjectRaw)
	if errors.Is(err, models.FormatValidationError) {
		http.Error(c.Writer, "Format validation error", http.StatusUnprocessableEntity) // 422
		return
	} else if err != nil {
		http.Error(c.Writer, "", http.StatusInternalServerError)
		return
	}

	// make a decision
	triggerObjectMap := make(map[string]interface{})
	err = json.Unmarshal(requestData.TriggerObjectRaw, &triggerObjectMap)
	if err != nil {
		http.Error(c.Writer, "", http.StatusUnprocessableEntity)
		return
	}
	ClientObject := models.ClientObject{TableName: models.TableName(requestData.TriggerObjectType), Data: triggerObjectMap}
	decisionUsecase := api.UsecasesWithCreds(c.Request).NewDecisionUsecase()

	decision, err := decisionUsecase.CreateDecision(c.Request.Context(), models.CreateDecisionInput{
		ScenarioId:              requestData.ScenarioId,
		ClientObject:            ClientObject,
		OrganizationId:          organizationId,
		PayloadStructWithReader: payload,
	}, logger)
	if errors.Is(err, models.NotFoundError) || errors.Is(err, models.BadParameterError) {
		presentError(c.Writer, c.Request, err)
		return
	} else if err != nil {
		logger.ErrorContext(c.Request.Context(), "Could not create a decision: \n"+err.Error())
		http.Error(c.Writer, "", http.StatusInternalServerError)
		return
	}
	c.JSON(http.StatusOK, dto.NewAPIDecision(decision))
}
