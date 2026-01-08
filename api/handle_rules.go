package api

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/usecases"
	"github.com/checkmarble/marble-backend/utils"
)

func handleListRules(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		iterationID := c.Query("scenarioIterationId")
		if iterationID == "" {
			c.Status(http.StatusBadRequest)
			return
		}

		usecase := usecasesWithCreds(ctx, uc).NewRuleUsecase()
		rules, err := usecase.ListRules(ctx, iterationID)
		if presentError(ctx, c, err) {
			return
		}

		apiRules, err := pure_utils.MapErr(rules, dto.AdaptRuleDto)
		if presentError(ctx, c, err) {
			return
		}
		c.JSON(http.StatusOK, apiRules)
	}
}

func handleCreateRule(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		organizationId, err := utils.OrganizationIdFromRequest(c.Request)
		if presentError(ctx, c, err) {
			return
		}

		var data dto.CreateRuleInputBody
		if err := c.ShouldBindJSON(&data); err != nil {
			c.Status(http.StatusBadRequest)
			return
		}

		createInputRule, err := dto.AdaptCreateRuleInput(data, organizationId)
		if presentError(ctx, c, err) {
			return
		}

		usecase := usecasesWithCreds(ctx, uc).NewRuleUsecase()
		rule, err := usecase.CreateRule(ctx, createInputRule)
		if handleExpectedIterationError(c, err) || presentError(ctx, c, err) {
			return
		}

		apiRule, err := dto.AdaptRuleDto(rule)
		if presentError(ctx, c, err) {
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"rule": apiRule,
		})
	}
}

func handleGetRule(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		ruleID := c.Param("rule_id")

		usecase := usecasesWithCreds(ctx, uc).NewRuleUsecase()
		rule, err := usecase.GetRule(ctx, ruleID)
		if presentError(ctx, c, err) {
			return
		}

		apiRule, err := dto.AdaptRuleDto(rule)
		if presentError(ctx, c, err) {
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"rule": apiRule,
		})
	}
}

func handleUpdateRule(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		ruleID := c.Param("rule_id")

		var data dto.UpdateRuleBody
		if err := c.ShouldBindJSON(&data); err != nil {
			c.Status(http.StatusBadRequest)
			return
		}

		updateRuleInput, err := dto.AdaptUpdateRule(ruleID, data)
		if presentError(ctx, c, err) {
			return
		}

		usecase := usecasesWithCreds(ctx, uc).NewRuleUsecase()
		updatedRule, err := usecase.UpdateRule(ctx, updateRuleInput)
		if handleExpectedIterationError(c, err) || presentError(ctx, c, err) {
			return
		}

		apiRule, err := dto.AdaptRuleDto(updatedRule)
		if presentError(ctx, c, err) {
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"rule": apiRule,
		})
	}
}

func handleDeleteRule(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		ruleID := c.Param("rule_id")

		usecase := usecasesWithCreds(ctx, uc).NewRuleUsecase()
		err := usecase.DeleteRule(ctx, ruleID)
		if presentError(ctx, c, err) {
			return
		}
		c.Status(http.StatusNoContent)
	}
}
