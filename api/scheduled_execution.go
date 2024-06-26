package api

import (
	"archive/zip"
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/utils"
)

func (api *API) handleGetScheduledExecution(c *gin.Context) {
	scheduledExecutionID := c.Param("execution_id")

	usecase := api.UsecasesWithCreds(c.Request).NewScheduledExecutionUsecase()
	execution, err := usecase.GetScheduledExecution(c.Request.Context(), scheduledExecutionID)

	if presentError(c, err) {
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"scheduled_execution": dto.AdaptScheduledExecutionDto(execution),
	})
}

func (api *API) handleGetScheduledExecutionDecisions(c *gin.Context) {
	scheduledExecutionID := c.Param("execution_id")
	organizationId, err := utils.OrganizationIdFromRequest(c.Request)
	if presentError(c, err) {
		return
	}

	usecase := api.UsecasesWithCreds(c.Request).NewScheduledExecutionUsecase()

	zipWriter := zip.NewWriter(c.Writer)
	defer zipWriter.Close()

	fileWriter, err := zipWriter.Create(fmt.Sprintf("decisions_of_execution_%s.ndjson", scheduledExecutionID))
	if err != nil {
		presentError(c, err)
		return
	}

	c.Writer.Header().Set("Content-Type", "application/zip")
	c.Writer.Header().Set("Content-Disposition", "attachment; filename=\"decisions.ndjson.zip\"")
	numberOfExportedDecisions, err := usecase.ExportScheduledExecutionDecisions(
		c.Request.Context(), organizationId, scheduledExecutionID, fileWriter)
	if err != nil {
		// note: un case of security error, the header has not been sent, so we can still send a 401
		presentError(c, err)
		return
	}
	// nice trailer
	c.Writer.Header().Set("X-NUMBER-OF-DECISIONS", strconv.Itoa(numberOfExportedDecisions))
}

func (api *API) handleListScheduledExecution(c *gin.Context) {
	scenarioID := c.Query("scenario_id")

	usecase := api.UsecasesWithCreds(c.Request).NewScheduledExecutionUsecase()
	executions, err := usecase.ListScheduledExecutions(c.Request.Context(), scenarioID)

	if presentError(c, err) {
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"scheduled_executions": pure_utils.Map(executions, dto.AdaptScheduledExecutionDto),
	})
}

func (api *API) handleCreateScheduledExecution(c *gin.Context) {
	ctx := c.Request.Context()
	organizationId, err := utils.OrgIDFromCtx(ctx, c.Request)
	if presentError(c, err) {
		return
	}

	iterationID := c.Param("iteration_id")

	usecase := api.UsecasesWithCreds(c.Request).NewScheduledExecutionUsecase()
	err = usecase.CreateScheduledExecution(c.Request.Context(), models.CreateScheduledExecutionInput{
		OrganizationId:      organizationId,
		ScenarioIterationId: iterationID,
	})

	if presentError(c, err) {
		return
	}
	c.Status(http.StatusCreated)
}
