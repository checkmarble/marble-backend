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

func handleGetScheduledExecution(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		scheduledExecutionID := c.Param("execution_id")

		usecase := usecasesWithCreds(c.Request, uc).NewScheduledExecutionUsecase()
		execution, err := usecase.GetScheduledExecution(c.Request.Context(), scheduledExecutionID)

		if presentError(c, err) {
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"scheduled_execution": dto.AdaptScheduledExecutionDto(execution),
		})
	}
}

func handleListScheduledExecution(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		scenarioId := c.Query("scenario_id")
		scenarioIdPtr := utils.PtrTo(scenarioId, &utils.PtrToOptions{OmitZero: true})

		organizationId, err := utils.OrganizationIdFromRequest(c.Request)
		if presentError(c, err) {
			return
		}

		usecase := usecasesWithCreds(c.Request, uc).NewScheduledExecutionUsecase()

		executions, err := usecase.ListScheduledExecutions(c.Request.Context(), organizationId, scenarioIdPtr)

		if presentError(c, err) {
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"scheduled_executions": pure_utils.Map(executions, dto.AdaptScheduledExecutionDto),
		})
	}
}

func handleCreateScheduledExecution(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		organizationId, err := utils.OrganizationIdFromRequest(c.Request)
		if presentError(c, err) {
			return
		}

		iterationID := c.Param("iteration_id")

		usecase := usecasesWithCreds(c.Request, uc).NewScheduledExecutionUsecase()
		err = usecase.CreateScheduledExecution(c.Request.Context(), models.CreateScheduledExecutionInput{
			OrganizationId:      organizationId,
			ScenarioIterationId: iterationID,
		})

		if presentError(c, err) {
			return
		}
		c.Status(http.StatusCreated)
	}
}
