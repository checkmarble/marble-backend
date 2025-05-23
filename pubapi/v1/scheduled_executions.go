package v1

import (
	"fmt"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pubapi"
	"github.com/checkmarble/marble-backend/pubapi/v1/dto"
	"github.com/checkmarble/marble-backend/pubapi/v1/params"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/usecases"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/gin-gonic/gin"
)

var batchExecutionsPaginationDefaults = models.PaginationDefaults{
	Limit:  25,
	SortBy: models.SortingFieldFrom("started_at"),
	Order:  models.SortingOrderDesc,
}

func HandleListBatchExecutions(uc usecases.Usecases) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		orgId, err := utils.OrganizationIdFromRequest(c.Request)
		if err != nil {
			pubapi.NewErrorResponse().WithError(err).Serve(c)
			return
		}

		var params params.ListBatchExecutionsParams

		if err := c.ShouldBindQuery(&params); err != nil {
			pubapi.NewErrorResponse().WithError(err).Serve(c)
			return
		}

		uc := pubapi.UsecasesWithCreds(ctx, uc)
		scheduledExecutionsUsecase := uc.NewScheduledExecutionUsecase()

		filters := params.ToFilters(orgId)
		paging := params.PaginationParams.ToModel(batchExecutionsPaginationDefaults)

		fmt.Println(filters)
		fmt.Println(params.PaginationParams)

		scheduledExecutions, err := scheduledExecutionsUsecase.ListScheduledExecutions(ctx, orgId, filters, &paging)
		if err != nil {
			pubapi.NewErrorResponse().WithError(err).Serve(c)
			return
		}

		var nextPageId string

		if scheduledExecutions.HasMore && len(scheduledExecutions.Executions) > 0 {
			nextPageId = scheduledExecutions.Executions[len(scheduledExecutions.Executions)-1].Id
		}

		pubapi.
			NewResponse(pure_utils.Map(scheduledExecutions.Executions, dto.AdaptScheduledExecution)).
			WithPagination(scheduledExecutions.HasMore, nextPageId).
			Serve(c)
	}
}
