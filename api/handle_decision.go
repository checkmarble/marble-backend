package api

import (
	"fmt"
	"net/http"

	"github.com/cockroachdb/errors"
	"github.com/gin-gonic/gin"

	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
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
	c.JSON(http.StatusOK, dto.NewAPIDecisionWithRule(decision, api.marbleAppHost, true))
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
	decisions, err := usecase.ListDecisions(
		c.Request.Context(),
		organizationId,
		dto.AdaptPaginationAndSortingInput(paginationAndSorting),
		filters,
	)
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
		"items": pure_utils.Map(decisions, func(d models.DecisionWithRank) dto.APIDecision {
			return dto.NewAPIDecision(d.Decision, api.marbleAppHost)
		}),
	})
}

func (api *API) handlePostDecision(c *gin.Context) {
	organizationId, err := utils.OrgIDFromCtx(c.Request.Context(), c.Request)
	if presentError(c, err) {
		return
	}

	var requestData dto.CreateDecisionWithScenarioBody
	if err := c.ShouldBindJSON(&requestData); err != nil {
		c.Status(http.StatusBadRequest)
		return
	}

	// make a decision
	decisionUsecase := api.UsecasesWithCreds(c.Request).NewDecisionUsecase()
	decision, err := decisionUsecase.CreateDecision(
		c.Request.Context(),
		models.CreateDecisionInput{
			OrganizationId:     organizationId,
			PayloadRaw:         requestData.TriggerObjectRaw,
			ScenarioId:         requestData.ScenarioId,
			TriggerObjectTable: requestData.TriggerObjectType,
		},
		false,
		true,
	)

	if returnExpectedDecisionError(c, err) || presentError(c, err) {
		return
	}
	c.JSON(http.StatusOK, dto.NewAPIDecisionWithRule(decision, api.marbleAppHost, false))
}

func returnExpectedDecisionError(c *gin.Context, err error) bool {
	if err == nil {
		return false
	}
	logger := utils.LoggerFromContext(c.Request.Context())
	logger.InfoContext(c.Request.Context(), fmt.Sprintf("error: %v", err))

	if errors.Is(err, models.ErrScenarioTriggerConditionAndTriggerObjectMismatch) {
		c.JSON(http.StatusBadRequest, dto.APIErrorResponse{
			Message:   "The payload object you sent does not match the trigger condition of the scenario.",
			ErrorCode: dto.CannotPublishDraft,
		})
		return true
	}
	return false
}

func (api *API) handlePostAllDecisions(c *gin.Context) {
	organizationId, err := utils.OrgIDFromCtx(c.Request.Context(), c.Request)
	if presentError(c, err) {
		return
	}

	var requestData dto.CreateDecisionBody
	if err := c.ShouldBindJSON(&requestData); err != nil {
		c.Status(http.StatusBadRequest)
		return
	}

	decisionUsecase := api.UsecasesWithCreds(c.Request).NewDecisionUsecase()
	decisions, nbSkipped, err := decisionUsecase.CreateAllDecisions(
		c.Request.Context(),
		models.CreateAllDecisionsInput{
			OrganizationId:     organizationId,
			PayloadRaw:         requestData.TriggerObjectRaw,
			TriggerObjectTable: requestData.TriggerObjectType,
		},
	)
	if presentError(c, err) {
		return
	}
	c.JSON(http.StatusOK, dto.AdaptAPIDecisionsWithMetadata(decisions, api.marbleAppHost, nbSkipped, false))
}
