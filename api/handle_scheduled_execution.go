package api

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/usecases"
	"github.com/checkmarble/marble-backend/utils"
)

func handleGetScheduledExecution(uc usecases.Usecaser) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		scheduledExecutionID := c.Param("execution_id")

		usecase := usecasesWithCreds(ctx, uc).NewScheduledExecutionUsecase()
		execution, err := usecase.GetScheduledExecution(ctx, scheduledExecutionID)

		if presentError(ctx, c, err) {
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"scheduled_execution": dto.AdaptScheduledExecutionDto(execution),
		})
	}
}

func handleListScheduledExecution(uc usecases.Usecaser) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		scenarioId := c.Query("scenario_id")
		scenarioIdPtr := utils.PtrTo(scenarioId, &utils.PtrToOptions{OmitZero: true})

		organizationId, err := utils.OrganizationIdFromRequest(c.Request)
		if presentError(ctx, c, err) {
			return
		}

		usecase := usecasesWithCreds(ctx, uc).NewScheduledExecutionUsecase()

		executions, err := usecase.ListScheduledExecutions(ctx, organizationId, scenarioIdPtr)

		if presentError(ctx, c, err) {
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"scheduled_executions": pure_utils.Map(executions, dto.AdaptScheduledExecutionDto),
		})
	}
}

func handleCreateScheduledExecution(uc usecases.Usecaser) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		organizationId, err := utils.OrganizationIdFromRequest(c.Request)
		if presentError(ctx, c, err) {
			return
		}

		iterationID := c.Param("iteration_id")

		usecase := usecasesWithCreds(ctx, uc).NewScheduledExecutionUsecase()
		err = usecase.CreateScheduledExecution(ctx, models.CreateScheduledExecutionInput{
			OrganizationId:      organizationId,
			ScenarioIterationId: iterationID,
		})

		if presentError(ctx, c, err) {
			return
		}
		c.Status(http.StatusCreated)
	}
}
