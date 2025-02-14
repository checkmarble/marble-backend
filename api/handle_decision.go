package api

import (
	"fmt"
	"net/http"

	"github.com/cockroachdb/errors"
	"github.com/gin-gonic/gin"

	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/usecases"
	"github.com/checkmarble/marble-backend/utils"
)

var decisionPaginationDefaults = models.PaginationDefaults{
	Limit:  25,
	SortBy: models.DecisionSortingCreatedAt,
	Order:  models.SortingOrderDesc,
}

func handleGetDecision(uc usecases.Usecases, marbleAppHost string) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		decisionID := c.Param("decision_id")

		usecase := usecasesWithCreds(ctx, uc).NewDecisionUsecase()
		decision, err := usecase.GetDecision(ctx, decisionID)
		if presentError(ctx, c, err) {
			return
		}
		c.JSON(http.StatusOK, dto.NewDecisionWithRuleDto(decision, marbleAppHost, true))
	}
}

// Endpoint used by the public API, that does not return the output decision ranks
func handleListDecisions(uc usecases.Usecases, marbleAppHost string) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		organizationId, err := utils.OrganizationIdFromRequest(c.Request)
		if presentError(ctx, c, err) {
			return
		}

		var filters dto.DecisionFilters
		if err := c.ShouldBind(&filters); err != nil {
			c.Status(http.StatusBadRequest)
			return
		}

		var paginationAndSortingDto dto.PaginationAndSorting
		if err := c.ShouldBind(&paginationAndSortingDto); err != nil {
			c.Status(http.StatusBadRequest)
			return
		}
		paginationAndSorting := models.WithPaginationDefaults(
			dto.AdaptPaginationAndSorting(paginationAndSortingDto), decisionPaginationDefaults)

		usecase := usecasesWithCreds(ctx, uc).NewDecisionUsecase()
		decisions, err := usecase.ListDecisions(ctx, organizationId, paginationAndSorting, filters)
		if presentError(ctx, c, err) {
			return
		}

		c.JSON(http.StatusOK, dto.AdaptDecisionListPageDto(decisions, marbleAppHost))
	}
}

// Endpoint used by the internal API to serve the app, that returns the output decision ranks
func handleListDecisionsInternal(uc usecases.Usecases, marbleAppHost string) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		organizationId, err := utils.OrganizationIdFromRequest(c.Request)
		if presentError(ctx, c, err) {
			return
		}

		var filters dto.DecisionFilters
		if err := c.ShouldBind(&filters); err != nil {
			c.Status(http.StatusBadRequest)
			return
		}

		var paginationAndSortingDto dto.PaginationAndSorting
		if err := c.ShouldBind(&paginationAndSortingDto); err != nil {
			c.Status(http.StatusBadRequest)
			return
		}
		paginationAndSorting := models.WithPaginationDefaults(
			dto.AdaptPaginationAndSorting(paginationAndSortingDto), decisionPaginationDefaults)

		usecase := usecasesWithCreds(ctx, uc).NewDecisionUsecase()
		decisions, err := usecase.ListDecisionsWithIndexes(ctx, organizationId, paginationAndSorting, filters)
		if presentError(ctx, c, err) {
			return
		}

		c.JSON(http.StatusOK, dto.AdaptDecisionListPageWithIndexesDto(decisions, marbleAppHost))
	}
}

func handlePostDecision(uc usecases.Usecases, marbleAppHost string) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		organizationId, err := utils.OrganizationIdFromRequest(c.Request)
		if presentError(ctx, c, err) {
			return
		}

		var requestData dto.CreateDecisionWithScenarioBody
		if err := c.ShouldBindJSON(&requestData); err != nil {
			c.Status(http.StatusBadRequest)
			return
		}

		// make a decision
		decisionUsecase := usecasesWithCreds(ctx, uc).NewDecisionUsecase()
		decision, err := decisionUsecase.CreateDecision(
			ctx,
			models.CreateDecisionInput{
				OrganizationId:     organizationId,
				PayloadRaw:         requestData.TriggerObject,
				ScenarioId:         requestData.ScenarioId,
				TriggerObjectTable: requestData.ObjectType,
			},
			models.CreateDecisionParams{
				WithScenarioPermissionCheck: true,
				WithDecisionWebhooks:        true,
				WithRuleExecutionDetails:    true,
			},
		)

		if presentIngestionValidationError(c, err) || returnExpectedDecisionError(c, err) || presentError(ctx, c, err) {
			return
		}
		c.JSON(http.StatusOK, dto.NewDecisionWithRuleDto(decision, marbleAppHost, false))
	}
}

func returnExpectedDecisionError(c *gin.Context, err error) bool {
	if err == nil {
		return false
	}
	ctx := c.Request.Context()
	logger := utils.LoggerFromContext(ctx)
	logger.InfoContext(ctx, fmt.Sprintf("error: %v", err))

	if errors.Is(err, models.ErrScenarioTriggerConditionAndTriggerObjectMismatch) {
		c.JSON(http.StatusBadRequest, dto.APIErrorResponse{
			Message:   "The payload object you sent does not match the trigger condition of the scenario.",
			ErrorCode: dto.TriggerConditionNotMatched,
		})
		return true
	}
	return false
}

func handlePostAllDecisions(uc usecases.Usecases, marbleAppHost string) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		organizationId, err := utils.OrganizationIdFromRequest(c.Request)
		if presentError(ctx, c, err) {
			return
		}

		var requestData dto.CreateDecisionBody
		if err := c.ShouldBindJSON(&requestData); err != nil {
			c.Status(http.StatusBadRequest)
			return
		}

		decisionUsecase := usecasesWithCreds(ctx, uc).NewDecisionUsecase()
		decisions, nbSkipped, err := decisionUsecase.CreateAllDecisions(
			ctx,
			models.CreateAllDecisionsInput{
				OrganizationId:     organizationId,
				PayloadRaw:         requestData.TriggerObject,
				TriggerObjectTable: requestData.ObjectType,
			},
		)
		if presentIngestionValidationError(c, err) || returnExpectedDecisionError(c, err) || presentError(ctx, c, err) {
			return
		}
		c.JSON(http.StatusOK, dto.AdaptDecisionsWithMetadataDto(decisions, marbleAppHost, nbSkipped, false))
	}
}
