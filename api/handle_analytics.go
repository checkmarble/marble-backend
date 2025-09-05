package api

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/usecases"
	"github.com/checkmarble/marble-backend/utils"
)

func handleListAnalytics(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		organizationId, err := utils.OrganizationIdFromRequest(c.Request)
		if presentError(ctx, c, err) {
			return
		}

		usecase := usecasesWithCreds(ctx, uc).NewAnalyticsUseCase()
		analytics, err := usecase.ListAnalytics(ctx, organizationId)
		if presentError(ctx, c, err) {
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"analytics": pure_utils.Map(analytics, dto.AdaptAnalyticsDto),
		})
	}
}

func handleAnalyticsQuery(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		uc := usecasesWithCreds(ctx, uc).NewAnalyticsQueryUsecase()

		var filters dto.AnalyticsQueryFilters

		if err := c.ShouldBindJSON(&filters); presentError(ctx, c, err) {
			c.Status(http.StatusBadRequest)
			return
		}
		if err := filters.Validate(); err != nil {
			c.Status(http.StatusBadRequest)
			return
		}

		var (
			results any
			err     error
		)

		switch c.Param("query") {
		case "decision_outcomes_per_day":
			results, err = uc.DecisionOutcomePerDay(c.Request.Context(), filters)
		case "decisions_score_distribution":
			results, err = uc.DecisionsScoreDistribution(c.Request.Context(), filters)
		case "rule_hit_table":
			results, err = uc.RuleHitTable(c.Request.Context(), filters)
		case "rule_vs_decision_outcome":
			results, err = uc.RuleVsDecisionOutcome(c.Request.Context(), filters)
		case "rule_cooccurence_matrix":
			results, err = uc.RuleCoOccurenceMatrix(c.Request.Context(), filters)
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

func handleAnalyticsAvailableFilters(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		uc := usecasesWithCreds(ctx, uc).NewAnalyticsMetadataUsecase()

		var req dto.AnalyticsAvailableFiltersRequest

		if err := c.ShouldBindJSON(&req); presentError(ctx, c, err) {
			c.Status(http.StatusBadRequest)
			return
		}

		filters, err := uc.GetAvailableFilters(c.Request.Context(), req)

		if presentError(ctx, c, err) {
			return
		}

		c.JSON(http.StatusOK, pure_utils.Map(filters, dto.AdaptAnalyticsAvailableFilter))
	}
}
