package v1

import (
	"github.com/checkmarble/marble-backend/pubapi"
	"github.com/checkmarble/marble-backend/pubapi/v1/dto"
	"github.com/checkmarble/marble-backend/pubapi/v1/params"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/usecases"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/gin-gonic/gin"
)

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

		scheduledExecutions, err := scheduledExecutionsUsecase.ListScheduledExecutions(ctx, orgId, params.ScenarioId)
		if err != nil {
			pubapi.NewErrorResponse().WithError(err).Serve(c)
			return
		}

		pubapi.NewResponse(pure_utils.Map(scheduledExecutions, dto.AdaptScheduledExecution)).Serve(c)
		// pubapi.NewResponse(scheduledExecutions).Serve(c)
	}
}
