package v1

import (
	"net/http"

	"github.com/checkmarble/marble-backend/pubapi"
	"github.com/checkmarble/marble-backend/pubapi/types"
	"github.com/checkmarble/marble-backend/pubapi/v1/dto"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/usecases"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/gin-gonic/gin"
)

func HandleCreateAsyncDecisions(uc usecases.Usecases) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		orgId, err := utils.OrganizationIdFromRequest(c.Request)
		if err != nil {
			types.NewErrorResponse().WithError(err).Serve(c)
			return
		}

		var payload dto.CreateAsyncDecisionParams

		if err := c.ShouldBindJSON(&payload); err != nil {
			types.NewErrorResponse().WithError(err).Serve(c)
			return
		}

		uc := pubapi.UsecasesWithCreds(ctx, uc)
		asyncUsecase := uc.NewAsyncDecisionExecutionUsecase()

		executions, err := asyncUsecase.CreateAsyncDecisionExecution(ctx,
			orgId, payload.TriggerObjectType, payload.TriggerObjects, payload.ScenarioId, payload.Ingest)
		if err != nil {
			if types.PresentMultipleObjectsValidationError(c, err) {
				return
			}

			types.NewErrorResponse().WithError(err).Serve(c)
			return
		}

		dtos := pure_utils.Map(executions, dto.AdaptAsyncDecisionExecutionCreated)

		types.NewResponse(dtos).Serve(c, http.StatusCreated)
	}
}

func HandleGetAsyncDecisionExecution(uc usecases.Usecases) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		executionId, err := types.UuidParam(c, "executionId")
		if err != nil {
			types.NewErrorResponse().WithError(err).Serve(c)
			return
		}

		uc := pubapi.UsecasesWithCreds(ctx, uc)
		asyncUsecase := uc.NewAsyncDecisionExecutionUsecase()

		execution, err := asyncUsecase.GetAsyncDecisionExecution(ctx, *executionId)
		if err != nil {
			types.NewErrorResponse().WithError(err).Serve(c)
			return
		}

		types.NewResponse(dto.AdaptAsyncDecisionExecution(execution)).Serve(c)
	}
}
