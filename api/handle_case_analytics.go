package api

import (
	"net/http"

	"github.com/cockroachdb/errors"
	"github.com/gin-gonic/gin"

	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/usecases"
	"github.com/checkmarble/marble-backend/utils"
)

func handleCaseAnalyticsQuery(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		var filters dto.CaseAnalyticsFilters
		if err := c.ShouldBindJSON(&filters); err != nil {
			presentError(ctx, c, errors.Wrap(models.BadParameterError, err.Error()))
			return
		}
		if err := filters.Validate(); err != nil {
			presentError(ctx, c, err)
			return
		}

		orgId, err := utils.OrganizationIdFromRequest(c.Request)
		if err != nil {
			presentError(ctx, c, errors.Wrap(models.UnAuthorizedError, err.Error()))
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
		case "cases_duration":
			results, err = uc.CasesDurationByTimeStats(ctx, filters)
		case "sar_completed":
			results, err = uc.SarCompletedCount(ctx, filters)
		case "open_cases_by_age":
			results, err = uc.OpenCasesByAge(ctx, filters)
		case "sar_delay":
			results, err = uc.SarDelayByTimeStats(ctx, filters)
		case "sar_delay_distribution":
			results, err = uc.SarDelayDistribution(ctx, filters)
		case "case_status_by_date":
			results, err = uc.CaseStatusByDate(ctx, filters)
		case "case_status_by_inbox":
			results, err = uc.CaseStatusByInbox(ctx, filters)
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
