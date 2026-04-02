package api

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/usecases"
	"github.com/checkmarble/marble-backend/utils"
)

func handleCaseAnalyticsQuery(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		var filters dto.CaseAnalyticsFilters
		if err := c.ShouldBindJSON(&filters); err != nil {
			c.Status(http.StatusBadRequest)
			return
		}
		if err := filters.Validate(); err != nil {
			c.Status(http.StatusBadRequest)
			return
		}

		orgId, err := utils.OrganizationIdFromRequest(c.Request)
		if err != nil {
			c.Status(http.StatusUnauthorized)
			return
		}
		filters.OrgId = orgId

		uc := usecasesWithCreds(ctx, uc).NewCaseAnalyticsUsecase()

		var results any

		switch c.Param("query") {
		case "cases_created":
			results, err = uc.CasesCreatedByTimeStats(ctx, filters)
		case "cases_false_positive_rate":
			results, err = uc.CasesFalsePositiveRateByTimeStats(ctx, filters)
		default:
			c.Status(http.StatusNotFound)
			return
		}

		if presentError(ctx, c, err) {
			return
		}

		c.JSON(http.StatusOK, results)
	}
}
