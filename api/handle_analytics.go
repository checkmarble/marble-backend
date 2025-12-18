package api

import (
	"cmp"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"

	"github.com/cockroachdb/errors"
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

		if err := c.ShouldBindJSON(&filters); err != nil {
			switch {
			case errors.Is(err, io.EOF):
				// No body is valid for some queries

			default:
				if presentError(ctx, c, err) {
					c.Status(http.StatusBadRequest)
					return
				}
			}
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

		var results any

		switch c.Param("query") {
		case "decision_outcomes_per_day":
			results, err = uc.DecisionOutcomePerDay(c.Request.Context(), filters)
		case "decisions_score_distribution":
			results, err = uc.DecisionsScoreDistribution(c.Request.Context(), filters)
			fmt.Println("OK")
		case "rule_hit_table":
			results, err = uc.RuleHitTable(c.Request.Context(), filters)
		case "rule_vs_decision_outcome":
			results, err = uc.RuleVsDecisionOutcome(c.Request.Context(), filters)
		case "rule_cooccurence_matrix":
			results, err = uc.RuleCoOccurenceMatrix(c.Request.Context(), filters)
		case "screening_hits":
			results, err = uc.ScreeningHits(c.Request.Context(), filters)

		// The following endpoint use Postgres, for now, instead of DuckDB.

		case "case_status_by_date":
			results, err = uc.CaseStatusByDate(c.Request.Context(), filters)
		case "case_status_by_inbox":
			results, err = uc.CaseStatusByInbox(c.Request.Context(), filters)

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
		if len(filters) == 0 {
			c.JSON(http.StatusOK, []struct{}{})
			return
		}

		fields := slices.SortedFunc(slices.Values(
			pure_utils.Map(filters, dto.AdaptAnalyticsAvailableFilter)), func(
			m1, m2 dto.AnalyticsAvailableFilter,
		) int {
			return cmp.Or(
				cmp.Compare(m1.Source, m2.Source),
				cmp.Compare(m1.Name, m2.Name),
			)
		})

		c.JSON(http.StatusOK, fields)
	}
}

func handleAnalyticsProxy(proxyApiUrl string) func(c *gin.Context) {
	proxyUrl, err := url.Parse(proxyApiUrl)
	if err != nil {
		proxyUrl = nil
	}

	return func(c *gin.Context) {
		ctx := c.Request.Context()

		if proxyUrl == nil {
			c.Status(http.StatusNotFound)
			return
		}

		c.Request.URL.Scheme = proxyUrl.Scheme
		c.Request.URL.Host = proxyUrl.Host

		req, err := http.NewRequestWithContext(ctx, c.Request.Method, c.Request.URL.String(), c.Request.Body)
		if presentError(ctx, c, err) {
			return
		}

		req.Header = c.Request.Header

		resp, err := http.DefaultClient.Do(req)
		if presentError(ctx, c, err) {
			return
		}

		if _, err := io.Copy(c.Writer, resp.Body); presentError(ctx, c, err) {
			return
		}

		c.Status(resp.StatusCode)

		for k, vs := range resp.Header {
			for _, v := range vs {
				c.Writer.Header().Add(k, v)
			}
		}
	}
}
