package v1

import (
	"fmt"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pubapi"
	"github.com/checkmarble/marble-backend/pubapi/v1/dto"
	"github.com/checkmarble/marble-backend/pubapi/v1/params"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/usecases"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/cockroachdb/errors"
	"github.com/gin-gonic/gin"
)

var decisionPaginationDefaults = models.PaginationDefaults{
	Limit:  25,
	SortBy: models.DecisionSortingCreatedAt,
	Order:  models.SortingOrderDesc,
}

func HandleListDecisions(uc usecases.Usecases) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		orgId, err := utils.OrganizationIdFromRequest(c.Request)
		if err != nil {
			pubapi.NewErrorResponse().WithError(err).Serve(c)
			return
		}

		var params params.ListDecisionsParams

		if err := c.ShouldBindQuery(&params); err != nil {
			pubapi.NewErrorResponse().WithError(err).Serve(c)
			return
		}

		if !params.StartDate.IsZero() && !params.EndDate.IsZero() {
			if time.Time(params.StartDate).After(time.Time(params.EndDate)) {
				pubapi.NewErrorResponse().WithError(errors.WithDetail(pubapi.ErrInvalidPayload, "end date should be after start date")).Serve(c)
				return
			}
		}

		filters := params.ToFilters()
		paging := models.WithPaginationDefaults(params.PaginationParams.ToModel(), decisionPaginationDefaults)

		uc := pubapi.UsecasesWithCreds(ctx, uc)
		decisionsUsecase := uc.NewDecisionUsecase()

		decisions, err := decisionsUsecase.ListDecisions(ctx, orgId, paging, filters)
		if err != nil {
			pubapi.NewErrorResponse().WithError(err).Serve(c)
			return
		}

		nextPageId := ""

		if len(decisions.Decisions) > 0 {
			nextPageId = decisions.Decisions[len(decisions.Decisions)-1].DecisionId
		}

		pubapi.
			NewResponse(pure_utils.Map(decisions.Decisions, dto.AdaptDecision(nil))).
			WithPagination(decisions.HasNextPage, nextPageId).
			Serve(c)
	}
}

func HandleGetDecision(uc usecases.Usecases) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		decisionId := c.Param("decisionId")

		uc := pubapi.UsecasesWithCreds(ctx, uc)
		decisionsUsecase := uc.NewDecisionUsecase()

		decision, err := decisionsUsecase.GetDecision(ctx, decisionId)
		fmt.Printf("%#v\n", decision)
		if err != nil {
			pubapi.NewErrorResponse().WithError(err).Serve(c)
			return
		}

		pubapi.
			NewResponse(dto.AdaptDecision(decision.RuleExecutions)(decision.Decision)).
			Serve(c)
	}
}
