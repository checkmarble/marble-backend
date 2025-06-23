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

		var payload dto.CreateWorkflowRuleDto

		if err := c.ShouldBindJSON(&payload); presentError(ctx, c, err) {
			c.Status(http.StatusBadRequest)
			return
		}

		uc := usecasesWithCreds(ctx, uc)
		scenarioUsecase := uc.NewScenarioUsecase()

		params := models.WorkflowRule{
			ScenarioId: payload.ScenarioId,
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
		ruleId := c.Param("ruleId")

		var payload dto.UpdateWorkflowRuleDto

		if err := c.ShouldBindJSON(&payload); presentError(ctx, c, err) {
			c.Status(http.StatusBadRequest)
			return
		}

		uc := usecasesWithCreds(ctx, uc)
		scenarioUsecase := uc.NewScenarioUsecase()

		params := models.WorkflowRule{
			Id:   ruleId,
			Name: payload.Name,
		}

		rule, err := scenarioUsecase.UpdateWorkflowRule(ctx, params)
		if presentError(ctx, c, err) {
			return
		}

		c.JSON(http.StatusCreated, dto.AdaptWorkflowRule(rule))
	}
}

func handleDeleteWorkflowRule(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		ruleId := c.Param("ruleId")

		uc := usecasesWithCreds(ctx, uc)
		scenarioUsecase := uc.NewScenarioUsecase()

		if err := scenarioUsecase.DeleteWorkflowRule(ctx, ruleId); presentError(ctx, c, err) {
			return
		}

		c.Status(http.StatusNoContent)
	}
}

func handleCreateWorkflowCondition(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		ruleId := c.Param("ruleId")

		var payload dto.PostWorkflowConditionDto

		if err := c.ShouldBindJSON(&payload); presentError(ctx, c, err) {
			c.Status(http.StatusBadRequest)
			return
		}
		if err := dto.ValidateWorkflowCondition(payload); presentError(ctx, c, err) {
			c.Status(http.StatusBadRequest)
			return
		}

		uc := usecasesWithCreds(ctx, uc)
		scenarioUsecase := uc.NewScenarioUsecase()

		params := models.WorkflowCondition{
			RuleId:   ruleId,
			Function: payload.Function,
			Params:   payload.Params,
		}

		condition, err := scenarioUsecase.CreateWorkflowCondition(ctx, params)
		if presentError(ctx, c, err) {
			return
		}

		c.JSON(http.StatusCreated, dto.AdaptWorkflowCondition(condition))
	}
}

func handleUpdateWorkflowCondition(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		ruleId := c.Param("ruleId")
		conditionId := c.Param("conditionId")

		var payload dto.PostWorkflowConditionDto

		if err := c.ShouldBindJSON(&payload); presentError(ctx, c, err) {
			c.Status(http.StatusBadRequest)
			return
		}
		if err := dto.ValidateWorkflowCondition(payload); presentError(ctx, c, err) {
			c.Status(http.StatusBadRequest)
			return
		}

		uc := usecasesWithCreds(ctx, uc)
		scenarioUsecase := uc.NewScenarioUsecase()

		params := models.WorkflowCondition{
			Id:       conditionId,
			RuleId:   ruleId,
			Function: payload.Function,
			Params:   payload.Params,
		}

		condition, err := scenarioUsecase.UpdateWorkflowCondition(ctx, params)
		if presentError(ctx, c, err) {
			return
		}

		c.JSON(http.StatusCreated, dto.AdaptWorkflowCondition(condition))
	}
}

func handleDeleteWorkflowCondition(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		ruleId := c.Param("ruleId")
		conditionId := c.Param("conditionId")

		uc := usecasesWithCreds(ctx, uc)
		scenarioUsecase := uc.NewScenarioUsecase()

		if err := scenarioUsecase.DeleteWorkflowCondition(ctx, ruleId, conditionId); presentError(ctx, c, err) {
			return
		}

		c.Status(http.StatusNoContent)
	}
}

func handleCreateWorkflowAction(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		ruleId := c.Param("ruleId")

		var payload dto.PostWorkflowActionDto

		if err := c.ShouldBindJSON(&payload); presentError(ctx, c, err) {
			return
		}
		if err := dto.ValidateWorkflowAction(payload); presentError(ctx, c, err) {
			c.Status(http.StatusBadRequest)
			return
		}

		uc := usecasesWithCreds(ctx, uc)
		scenarioUsecase := uc.NewScenarioUsecase()

		params := models.WorkflowAction{
			RuleId: ruleId,
			Action: payload.Action,
			Params: payload.Params,
		}

		action, err := scenarioUsecase.CreateWorkflowAction(ctx, params)
		if presentError(ctx, c, err) {
			return
		}

		c.JSON(http.StatusCreated, dto.AdaptWorkflowAction(action))
	}
}

func handleUpdateWorkflowAction(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		ruleId := c.Param("ruleId")
		actionId := c.Param("actionId")

		var payload dto.PostWorkflowActionDto

		if err := c.ShouldBindJSON(&payload); presentError(ctx, c, err) {
			return
		}
		if err := dto.ValidateWorkflowAction(payload); presentError(ctx, c, err) {
			c.Status(http.StatusBadRequest)
			return
		}

		uc := usecasesWithCreds(ctx, uc)
		scenarioUsecase := uc.NewScenarioUsecase()

		params := models.WorkflowAction{
			Id:     actionId,
			RuleId: ruleId,
			Action: payload.Action,
			Params: payload.Params,
		}

		action, err := scenarioUsecase.UpdateWorkflowAction(ctx, params)
		if presentError(ctx, c, err) {
			return
		}

		c.JSON(http.StatusCreated, dto.AdaptWorkflowAction(action))
	}
}
func handleDeleteWorkflowAction(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		ruleId := c.Param("ruleId")
		actionId := c.Param("actionId")

		uc := usecasesWithCreds(ctx, uc)
		scenarioUsecase := uc.NewScenarioUsecase()

		if err := scenarioUsecase.DeleteWorkflowAction(ctx, ruleId, actionId); presentError(ctx, c, err) {
			return
		}

		c.Status(http.StatusNoContent)
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
