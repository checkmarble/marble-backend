package api

import (
	"encoding/json"

	"fmt"
	"net/http"

	"github.com/cockroachdb/errors"
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
	decision, err := usecase.GetDecision(c.Request.Context(), decisionID)
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
	decisions, err := usecase.ListDecisions(c.Request.Context(), organizationId, dto.AdaptPaginationAndSortingInput(paginationAndSorting), filters)
	if presentError(c, err) {
		return
	}

	if len(decisions) == 0 {
		c.JSON(http.StatusOK, gin.H{
			"total_count": dto.AdaptTotalCount(models.TotalCount{}),
			"start_index": 0,
			"end_index":   0,
			"items":       []dto.APIDecision{},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"total_count": dto.AdaptTotalCount(decisions[0].TotalCount),
		"start_index": decisions[0].RankNumber,
		"end_index":   decisions[len(decisions)-1].RankNumber,
		"items":       utils.Map(decisions, func(d models.DecisionWithRank) dto.APIDecision { return dto.NewAPIDecision(d.Decision) }),
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
	dataModel, err := dataModelUseCase.GetDataModel(c.Request.Context(), organizationId)
	if err != nil {
		http.Error(c.Writer, "No data model found for organization", http.StatusInternalServerError)
		return
	}

	tables := dataModel.Tables
	table, ok := tables[models.TableName(requestData.TriggerObjectType)]
	if !ok {
		http.Error(c.Writer, fmt.Sprintf("Table %s not found", requestData.TriggerObjectType), http.StatusNotFound)
		return
	}

	parser := payload_parser.NewParser()
	validationErrors, err := parser.ValidatePayload(table, requestData.TriggerObjectRaw)
	if err != nil {
		presentError(c, errors.Wrap(models.BadParameterError, fmt.Sprintf("Error while validating payload: %v", err)))
		return
	}
	if len(validationErrors) > 0 {
		encoded, _ := json.Marshal(validationErrors)
		logger.InfoContext(c.Request.Context(), fmt.Sprintf("Validation errors on POST decisions: %s", string(encoded)))
		http.Error(c.Writer, string(encoded), http.StatusBadRequest)
		return
	}

	payload, err := payload_parser.ParseToDataModelObject(table, requestData.TriggerObjectRaw)
	if errors.Is(err, models.FormatValidationError) {
		errString := fmt.Sprintf("Format validation error: %v", err)
		logger.InfoContext(c.Request.Context(), errString)
		http.Error(c.Writer, errString, http.StatusUnprocessableEntity) // 422
		return
	} else if err != nil {
		presentError(c, errors.Wrap(err, "Error parsing payload in handlePostDecision"))
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
		presentError(c, errors.Wrap(err, "Error creating decision in handlePostDecision"))
		return
	}
	c.JSON(http.StatusOK, dto.NewAPIDecision(decision))
}
