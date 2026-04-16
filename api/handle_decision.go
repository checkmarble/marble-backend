package api

import (
	"net/http"
	"net/url"

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

func handleGetDecision(uc usecases.Usecases, marbleAppUrl *url.URL) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		decisionID := c.Param("decision_id")

		usecase := usecasesWithCreds(ctx, uc).NewDecisionUsecase()
		decision, err := usecase.GetDecision(ctx, decisionID)
		if presentError(ctx, c, err) {
			return
		}
		c.JSON(http.StatusOK, dto.NewDecisionWithRuleDto(decision, marbleAppUrl, true))
	}
}

func handleListDecisions(uc usecases.Usecases, marbleAppUrl *url.URL) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		organizationId, err := utils.OrganizationIdFromRequest(c.Request)
		if presentError(ctx, c, err) {
			return
		}

		var filters dto.DecisionFilters
		if err := c.ShouldBind(&filters); err != nil {
			c.JSON(http.StatusBadRequest, dto.APIErrorResponse{
				Message: err.Error(),
			})
			return
		}

		var paginationAndSortingDto dto.PaginationAndSorting
		if err := c.ShouldBind(&paginationAndSortingDto); err != nil {
			c.JSON(http.StatusBadRequest, dto.APIErrorResponse{
				Message: err.Error(),
			})
			return
		}
		paginationAndSorting := models.WithPaginationDefaults(
			dto.AdaptPaginationAndSorting(paginationAndSortingDto), decisionPaginationDefaults)

		usecase := usecasesWithCreds(ctx, uc).NewDecisionUsecase()
		decisions, err := usecase.ListDecisions(ctx, organizationId, paginationAndSorting, filters)
		if presentError(ctx, c, err) {
			return
		}

		c.JSON(http.StatusOK, dto.AdaptDecisionListPageDto(decisions, marbleAppUrl))
	}
}
