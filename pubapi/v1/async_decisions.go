package v1

import (
	"net/http"

	"github.com/checkmarble/marble-backend/pubapi"
	"github.com/checkmarble/marble-backend/pubapi/types"
	"github.com/checkmarble/marble-backend/pubapi/v1/dto"
	"github.com/checkmarble/marble-backend/pubapi/v1/params"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/usecases"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/gin-gonic/gin"
)

func HandleCreateAsyncDecision(uc usecases.Usecases) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		orgId, err := utils.OrganizationIdFromRequest(c.Request)
		if err != nil {
			types.NewErrorResponse().WithError(err).Serve(c)
			return
		}

		var payload params.CreateAsyncDecisionParams

		if err := c.ShouldBindJSON(&payload); err != nil {
			types.NewErrorResponse().WithError(err).Serve(c)
			return
		}

		uc := pubapi.UsecasesWithCreds(ctx, uc)
		asyncUsecase := uc.NewAsyncDecisionExecutionUsecase()

		execution, err := asyncUsecase.CreateAsyncDecisionExecution(ctx,
			orgId, payload.TriggerObjectType, payload.TriggerObject, payload.ScenarioId, payload.Ingest)
		if err != nil {
			if presentDecisionCreationError(c, err) {
				return
			}

			types.NewErrorResponse().WithError(err).Serve(c)
			return
		}

		types.NewResponse(dto.AdaptAsyncDecisionExecutionCreated(execution)).
			Serve(c, http.StatusCreated)
	}
}

func HandleCreateAsyncDecisionBatch(uc usecases.Usecases) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		orgId, err := utils.OrganizationIdFromRequest(c.Request)
		if err != nil {
			types.NewErrorResponse().WithError(err).Serve(c)
			return
		}

		var payload params.CreateAsyncDecisionBatchParams

		if err := c.ShouldBindJSON(&payload); err != nil {
			types.NewErrorResponse().WithError(err).Serve(c)
			return
		}

		uc := pubapi.UsecasesWithCreds(ctx, uc)
		asyncUsecase := uc.NewAsyncDecisionExecutionUsecase()

		executions, err := asyncUsecase.CreateAsyncDecisionExecutionBatch(ctx,
			orgId, payload.TriggerObjectType, payload.Objects, payload.ScenarioId, payload.Ingest)
		if err != nil {
			if presentDecisionCreationError(c, err) {
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

		orgId, err := utils.OrganizationIdFromRequest(c.Request)
		if err != nil {
			types.NewErrorResponse().WithError(err).Serve(c)
			return
		}

		executionId, err := types.UuidParam(c, "executionId")
		if err != nil {
			types.NewErrorResponse().WithError(err).Serve(c)
			return
		}

		uc := pubapi.UsecasesWithCreds(ctx, uc)
		asyncUsecase := uc.NewAsyncDecisionExecutionUsecase()

		execution, err := asyncUsecase.GetAsyncDecisionExecution(ctx, orgId, *executionId)
		if err != nil {
			types.NewErrorResponse().WithError(err).Serve(c)
			return
		}

		types.NewResponse(dto.AdaptAsyncDecisionExecution(execution)).Serve(c)
	}
}
