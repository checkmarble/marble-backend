package api

import (
	"net/http"

	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/usecases"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func handleListWorkflowsForScenario(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		scenarioId := c.Param("scenarioId")

		uc := usecasesWithCreds(ctx, uc)
		scenarioUsecase := uc.NewScenarioUsecase()

		rules, err := scenarioUsecase.ListWorkflowsForScenario(ctx, scenarioId)
		if presentError(ctx, c, err) {
			return
		}

		c.JSON(http.StatusOK, pure_utils.Map(rules, dto.AdaptWorkflow))
	}
}

func handleCreateWorkflowRule(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		scenarioId := c.Param("scenarioId")

		var payload dto.PostWorkflowRuleDto

		if err := c.ShouldBindJSON(&payload); presentError(ctx, c, err) {
			return
		}

		uc := usecasesWithCreds(ctx, uc)
		scenarioUsecase := uc.NewScenarioUsecase()

		params := models.WorkflowRule{
			ScenarioId: scenarioId,
			Name:       payload.Name,
		}

		rule, err := scenarioUsecase.CreateWorkflowRule(ctx, params)
		if presentError(ctx, c, err) {
			return
		}

		c.JSON(http.StatusCreated, dto.AdaptWorkflowRule(rule))
	}
}

func handleUpdateWorkflowRule(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		scenarioId := c.Param("scenarioId")
		ruleId := c.Param("ruleId")

		var payload dto.PostWorkflowRuleDto

		if err := c.ShouldBindJSON(&payload); presentError(ctx, c, err) {
			return
		}

		uc := usecasesWithCreds(ctx, uc)
		scenarioUsecase := uc.NewScenarioUsecase()

		params := models.WorkflowRule{
			Id:         ruleId,
			ScenarioId: scenarioId,
			Name:       payload.Name,
		}

		rule, err := scenarioUsecase.UpdateWorkflowRule(ctx, params)
		if presentError(ctx, c, err) {
			return
		}

		c.JSON(http.StatusCreated, dto.AdaptWorkflowRule(rule))
	}
}

func handleReorderWorkflowRules(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		scenarioId := c.Param("scenarioId")

		var ids []uuid.UUID

		if err := c.ShouldBindJSON(&ids); presentError(ctx, c, err) {
			return
		}

		uc := usecasesWithCreds(ctx, uc)
		scenarioUsecase := uc.NewScenarioUsecase()

		if err := scenarioUsecase.ReorderWorkflowRules(ctx, scenarioId, ids); presentError(ctx, c, err) {
			return
		}

		c.Status(http.StatusNoContent)
	}
}
