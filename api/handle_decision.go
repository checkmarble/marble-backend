package api

import (
	"fmt"
	"net/http"

	"github.com/cockroachdb/errors"
	"github.com/gin-gonic/gin"

	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/usecases"
	"github.com/checkmarble/marble-backend/utils"
)

var decisionPaginationDefaults = dto.PaginationDefaults{
	Limit:  25,
	SortBy: models.DecisionSortingCreatedAt,
	Order:  models.SortingOrderDesc,
}

func handleGetDecision(uc usecases.Usecases, marbleAppHost string) func(c *gin.Context) {
	return func(c *gin.Context) {
		decisionID := c.Param("decision_id")

		usecase := usecasesWithCreds(c.Request, uc).NewDecisionUsecase()
		decision, err := usecase.GetDecision(c.Request.Context(), decisionID)
		if presentError(c, err) {
			return
		}
		c.JSON(http.StatusOK, dto.NewDecisionWithRuleDto(decision, marbleAppHost, true))
	}
}

func handleListDecisions(uc usecases.Usecases, marbleAppHost string) func(c *gin.Context) {
	return func(c *gin.Context) {
		organizationId, err := utils.OrganizationIdFromRequest(c.Request)
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

		usecase := usecasesWithCreds(c.Request, uc).NewDecisionUsecase()
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
				"items":       []dto.Decision{},
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"total_count": dto.AdaptTotalCount(decisions[0].TotalCount),
			"start_index": decisions[0].RankNumber,
			"end_index":   decisions[len(decisions)-1].RankNumber,
			"items": pure_utils.Map(decisions, func(d models.DecisionWithRank) dto.Decision {
				return dto.NewDecisionDto(d.Decision, marbleAppHost)
			}),
		})
	}
}

func handlePostDecision(uc usecases.Usecases, marbleAppHost string) func(c *gin.Context) {
	return func(c *gin.Context) {
		organizationId, err := utils.OrganizationIdFromRequest(c.Request)
		if presentError(c, err) {
			return
		}

		var requestData dto.CreateDecisionWithScenarioBody
		if err := c.ShouldBindJSON(&requestData); err != nil {
			c.Status(http.StatusBadRequest)
			return
		}

		// make a decision
		decisionUsecase := usecasesWithCreds(c.Request, uc).NewDecisionUsecase()
		decision, err := decisionUsecase.CreateDecision(
			c.Request.Context(),
			models.CreateDecisionInput{
				OrganizationId:     organizationId,
				PayloadRaw:         requestData.TriggerObject,
				ScenarioId:         requestData.ScenarioId,
				TriggerObjectTable: requestData.ObjectType,
			},
			false,
			true,
		)

		if returnExpectedDecisionError(c, err) || presentError(c, err) {
			return
		}
		c.JSON(http.StatusOK, dto.NewDecisionWithRuleDto(decision, marbleAppHost, false))
	}
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

func handlePostAllDecisions(uc usecases.Usecases, marbleAppHost string) func(c *gin.Context) {
	return func(c *gin.Context) {
		organizationId, err := utils.OrganizationIdFromRequest(c.Request)
		if presentError(c, err) {
			return
		}

		var requestData dto.CreateDecisionBody
		if err := c.ShouldBindJSON(&requestData); err != nil {
			c.Status(http.StatusBadRequest)
			return
		}

		decisionUsecase := usecasesWithCreds(c.Request, uc).NewDecisionUsecase()
		decisions, nbSkipped, err := decisionUsecase.CreateAllDecisions(
			c.Request.Context(),
			models.CreateAllDecisionsInput{
				OrganizationId:     organizationId,
				PayloadRaw:         requestData.TriggerObject,
				TriggerObjectTable: requestData.ObjectType,
			},
		)
		if presentError(c, err) {
			return
		}
		c.JSON(http.StatusOK, dto.AdaptDecisionsWithMetadataDto(decisions, marbleAppHost, nbSkipped, false))
	}
}
