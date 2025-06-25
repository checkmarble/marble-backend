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

type ScenarioWorkflowParams struct {
	ScenarioId dto.UriUuid `uri:"scenarioId"`
}

type WorkflowRuleParams struct {
	RuleId dto.UriUuid `uri:"ruleId"`
	Id     dto.UriUuid `uri:"id"`
}

func handleListWorkflowsForScenario(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		var uri ScenarioWorkflowParams

		if err := c.ShouldBindUri(&uri); presentError(ctx, c, err) {
			c.Status(http.StatusBadRequest)
			return
		}

		uc := usecasesWithCreds(ctx, uc)
		workflowUsecase := uc.NewWorkflowUsecase()

		rules, err := workflowUsecase.ListWorkflowsForScenario(ctx, uri.ScenarioId.Uuid())
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
		workflowUsecase := uc.NewWorkflowUsecase()

		params := models.WorkflowRule{
			ScenarioId: payload.ScenarioId,
			Name:       payload.Name,
		}

		rule, err := workflowUsecase.CreateWorkflowRule(ctx, params)
		if presentError(ctx, c, err) {
			return
		}

		c.JSON(http.StatusCreated, dto.AdaptWorkflowRule(rule))
	}
}

func handleUpdateWorkflowRule(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		var (
			uri     WorkflowRuleParams
			payload dto.UpdateWorkflowRuleDto
		)

		if err := c.ShouldBindUri(&uri); presentError(ctx, c, err) {
			c.Status(http.StatusBadRequest)
			return
		}
		if err := c.ShouldBindJSON(&payload); presentError(ctx, c, err) {
			c.Status(http.StatusBadRequest)
			return
		}

		uc := usecasesWithCreds(ctx, uc)
		workflowUsecase := uc.NewWorkflowUsecase()

		params := models.WorkflowRule{
			Id:   uri.RuleId.Uuid(),
			Name: payload.Name,
		}

		rule, err := workflowUsecase.UpdateWorkflowRule(ctx, params)
		if presentError(ctx, c, err) {
			return
		}

		c.JSON(http.StatusCreated, dto.AdaptWorkflowRule(rule))
	}
}

func handleDeleteWorkflowRule(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		var uri WorkflowRuleParams

		if err := c.ShouldBindUri(&uri); presentError(ctx, c, err) {
			c.Status(http.StatusBadRequest)
			return
		}

		uc := usecasesWithCreds(ctx, uc)
		workflowUsecase := uc.NewWorkflowUsecase()

		if err := workflowUsecase.DeleteWorkflowRule(ctx, uri.RuleId.Uuid()); presentError(ctx, c, err) {
			return
		}

		c.Status(http.StatusNoContent)
	}
}

func handleCreateWorkflowCondition(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		var (
			uri     WorkflowRuleParams
			payload dto.PostWorkflowConditionDto
		)

		if err := c.ShouldBindJSON(&payload); presentError(ctx, c, err) {
			c.Status(http.StatusBadRequest)
			return
		}
		if err := c.ShouldBindUri(&uri); presentError(ctx, c, err) {
			c.Status(http.StatusBadRequest)
			return
		}

		uc := usecasesWithCreds(ctx, uc)
		workflowUsecase := uc.NewWorkflowUsecase()

		params := models.WorkflowCondition{
			RuleId:   uri.RuleId.Uuid(),
			Function: payload.Function,
			Params:   payload.Params,
		}

		condition, err := workflowUsecase.CreateWorkflowCondition(ctx, params)
		if presentError(ctx, c, err) {
			return
		}

		c.JSON(http.StatusCreated, dto.AdaptWorkflowCondition(condition))
	}
}

func handleUpdateWorkflowCondition(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		var (
			uri     WorkflowRuleParams
			payload dto.PostWorkflowConditionDto
		)

		if err := c.ShouldBindUri(&uri); presentError(ctx, c, err) {
			c.Status(http.StatusBadRequest)
			return
		}
		if err := c.ShouldBindJSON(&payload); presentError(ctx, c, err) {
			c.Status(http.StatusBadRequest)
			return
		}

		uc := usecasesWithCreds(ctx, uc)
		workflowUsecase := uc.NewWorkflowUsecase()

		params := models.WorkflowCondition{
			Id:       uri.Id.Uuid(),
			RuleId:   uri.RuleId.Uuid(),
			Function: payload.Function,
			Params:   payload.Params,
		}

		condition, err := workflowUsecase.UpdateWorkflowCondition(ctx, params)
		if presentError(ctx, c, err) {
			return
		}

		c.JSON(http.StatusCreated, dto.AdaptWorkflowCondition(condition))
	}
}

func handleDeleteWorkflowCondition(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		var uri WorkflowRuleParams

		if err := c.ShouldBindUri(&uri); presentError(ctx, c, err) {
			c.Status(http.StatusBadRequest)
			return
		}

		uc := usecasesWithCreds(ctx, uc)
		workflowUsecase := uc.NewWorkflowUsecase()

		if err := workflowUsecase.DeleteWorkflowCondition(ctx, uri.RuleId.Uuid(), uri.Id.Uuid()); presentError(ctx, c, err) {
			return
		}

		c.Status(http.StatusNoContent)
	}
}

func handleCreateWorkflowAction(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		var (
			uri     WorkflowRuleParams
			payload dto.PostWorkflowActionDto
		)

		if err := c.ShouldBindUri(&uri); presentError(ctx, c, err) {
			c.Status(http.StatusBadRequest)
			return
		}
		if err := c.ShouldBindJSON(&payload); presentError(ctx, c, err) {
			return
		}

		uc := usecasesWithCreds(ctx, uc)
		workflowUsecase := uc.NewWorkflowUsecase()

		params := models.WorkflowAction{
			RuleId: uri.RuleId.Uuid(),
			Action: payload.Action,
			Params: payload.Params,
		}

		action, err := workflowUsecase.CreateWorkflowAction(ctx, params)
		if presentError(ctx, c, err) {
			return
		}

		c.JSON(http.StatusCreated, dto.AdaptWorkflowAction(action))
	}
}

func handleUpdateWorkflowAction(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		var (
			uri     WorkflowRuleParams
			payload dto.PostWorkflowActionDto
		)

		if err := c.ShouldBindUri(&uri); presentError(ctx, c, err) {
			c.Status(http.StatusBadRequest)
			return
		}
		if err := c.ShouldBindJSON(&payload); presentError(ctx, c, err) {
			return
		}

		uc := usecasesWithCreds(ctx, uc)
		workflowUsecase := uc.NewWorkflowUsecase()

		params := models.WorkflowAction{
			Id:     uri.Id.Uuid(),
			RuleId: uri.RuleId.Uuid(),
			Action: payload.Action,
			Params: payload.Params,
		}

		action, err := workflowUsecase.UpdateWorkflowAction(ctx, params)
		if presentError(ctx, c, err) {
			return
		}

		c.JSON(http.StatusCreated, dto.AdaptWorkflowAction(action))
	}
}

func handleDeleteWorkflowAction(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		var uri WorkflowRuleParams

		if err := c.ShouldBindUri(&uri); presentError(ctx, c, err) {
			c.Status(http.StatusBadRequest)
			return
		}

		uc := usecasesWithCreds(ctx, uc)
		workflowUsecase := uc.NewWorkflowUsecase()

		if err := workflowUsecase.DeleteWorkflowAction(ctx, uri.RuleId.Uuid(), uri.Id.Uuid()); presentError(ctx, c, err) {
			return
		}

		c.Status(http.StatusNoContent)
	}
}

func handleReorderWorkflowRules(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		var (
			uri ScenarioWorkflowParams
			ids []uuid.UUID
		)

		if err := c.ShouldBindUri(&uri); presentError(ctx, c, err) {
			c.Status(http.StatusBadRequest)
			return
		}
		if err := c.ShouldBindJSON(&ids); presentError(ctx, c, err) {
			return
		}

		uc := usecasesWithCreds(ctx, uc)
		workflowUsecase := uc.NewWorkflowUsecase()

		if err := workflowUsecase.ReorderWorkflowRules(ctx, uri.ScenarioId.Uuid(), ids); presentError(ctx, c, err) {
			return
		}

		c.Status(http.StatusNoContent)
	}
}
