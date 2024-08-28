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
		iterationID := c.Query("scenarioIterationId")
		if iterationID == "" {
			c.Status(http.StatusBadRequest)
			return
		}

		usecase := usecasesWithCreds(c.Request, uc).NewRuleUsecase()
		rules, err := usecase.ListRules(c.Request.Context(), iterationID)
		if presentError(c, err) {
			return
		}

		apiRules, err := pure_utils.MapErr(rules, dto.AdaptRuleDto)
		if presentError(c, err) {
			return
		}
		c.JSON(http.StatusOK, apiRules)
	}
}

func handleCreateRule(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		organizationId, err := utils.OrganizationIdFromRequest(c.Request)
		if presentError(c, err) {
			return
		}

		var data dto.CreateRuleInputBody
		if err := c.ShouldBindJSON(&data); err != nil {
			c.Status(http.StatusBadRequest)
			return
		}

		createInputRule, err := dto.AdaptCreateRuleInput(data, organizationId)
		if presentError(c, err) {
			return
		}

		usecase := usecasesWithCreds(c.Request, uc).NewRuleUsecase()
		rule, err := usecase.CreateRule(c.Request.Context(), createInputRule)
		if handleExpectedIterationError(c, err) || presentError(c, err) {
			return
		}

		apiRule, err := dto.AdaptRuleDto(rule)
		if presentError(c, err) {
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"rule": apiRule,
		})
	}
}

func handleGetRule(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ruleID := c.Param("rule_id")

		usecase := usecasesWithCreds(c.Request, uc).NewRuleUsecase()
		rule, err := usecase.GetRule(c.Request.Context(), ruleID)
		if presentError(c, err) {
			return
		}

		apiRule, err := dto.AdaptRuleDto(rule)
		if presentError(c, err) {
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"rule": apiRule,
		})
	}
}

func handleUpdateRule(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ruleID := c.Param("rule_id")

		var data dto.UpdateRuleBody
		if err := c.ShouldBindJSON(&data); err != nil {
			c.Status(http.StatusBadRequest)
			return
		}

		updateRuleInput, err := dto.AdaptUpdateRule(ruleID, data)
		if presentError(c, err) {
			return
		}

		usecase := usecasesWithCreds(c.Request, uc).NewRuleUsecase()
		updatedRule, err := usecase.UpdateRule(c.Request.Context(), updateRuleInput)
		if handleExpectedIterationError(c, err) || presentError(c, err) {
			return
		}

		apiRule, err := dto.AdaptRuleDto(updatedRule)
		if presentError(c, err) {
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"rule": apiRule,
		})
	}
}

func handleDeleteRule(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ruleID := c.Param("rule_id")

		usecase := usecasesWithCreds(c.Request, uc).NewRuleUsecase()
		err := usecase.DeleteRule(c.Request.Context(), ruleID)
		if presentError(c, err) {
			return
		}
		c.Status(http.StatusNoContent)
	}
}
