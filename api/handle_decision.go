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

var decisionPaginationDefaults = dto.PaginationDefaults{
	Limit:  25,
	SortBy: models.DecisionSortingCreatedAt,
	Order:  models.SortingOrderDesc,
}

func (api *API) handleGetDecision(c *gin.Context) {
	decisionID := c.Param("decision_id")

	usecase := api.UsecasesWithCreds(c.Request).NewDecisionUsecase()
	decision, err := usecase.GetDecision(decisionID)
	if presentError(c, err) {
		return
	}
	c.JSON(http.StatusOK, dto.NewAPIDecision(decision))
}

func (api *API) handleListDecisions(c *gin.Context) {
	organizationId, err := utils.OrgIDFromCtx(c.Request.Context(), c.Request)
	if presentError(c, err) {
		return
	}

	var filters dto.DecisionFilters
	if err := c.ShouldBind(&filters); err != nil {
		c.Status(http.StatusBadRequest)
		return
	}

	var paginationAndSorting dto.PaginationAndSortingInput
	if err := c.ShouldBind(&paginationAndSorting); err != nil {
		c.Status(http.StatusBadRequest)
		return
	}
	paginationAndSorting = dto.WithPaginationDefaults(paginationAndSorting, decisionPaginationDefaults)

	usecase := api.UsecasesWithCreds(c.Request).NewDecisionUsecase()
	decisions, err := usecase.ListDecisions(organizationId, dto.AdaptPaginationAndSortingInput(paginationAndSorting), filters)
	if presentError(c, err) {
		return
	}

	if len(decisions) == 0 {
		c.JSON(http.StatusOK, gin.H{
			"total":      0,
			"startIndex": 0,
			"endIndex":   0,
			"items":      []dto.APIDecision{},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"total":      decisions[0].Total,
		"startIndex": decisions[0].RankNumber,
		"endIndex":   decisions[len(decisions)-1].RankNumber,
		"items":      utils.Map(decisions, func(d models.DecisionWithRank) dto.APIDecision { return dto.NewAPIDecision(d.Decision) }),
	})
}

func (api *API) handlePostDecision(c *gin.Context) {
	logger := utils.LoggerFromContext(c.Request.Context())

	organizationId, err := utils.OrgIDFromCtx(c.Request.Context(), c.Request)
	if presentError(c, err) {
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

	parser := payload_parser.NewParser()
	validationErrors, err := parser.ValidatePayload(table, requestData.TriggerObjectRaw)
	if err != nil {
		http.Error(c.Writer, "", http.StatusUnprocessableEntity)
		return
	}
	if len(validationErrors) > 0 {
		c.Writer.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(c.Writer).Encode(validationErrors)
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
		presentError(c, err)
		return
	} else if err != nil {
		logger.ErrorContext(c.Request.Context(), "Could not create a decision: \n"+err.Error())
		http.Error(c.Writer, "", http.StatusInternalServerError)
		return
	}
	c.JSON(http.StatusOK, dto.NewAPIDecision(decision))
}
