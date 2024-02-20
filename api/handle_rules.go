package api

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/utils"
)

func (api *API) ListRules(c *gin.Context) {
	iterationID := c.Query("scenarioIterationId")
	if iterationID == "" {
		c.Status(http.StatusBadRequest)
		return
	}

	usecase := api.UsecasesWithCreds(c.Request).NewRuleUsecase()
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

func (api *API) CreateRule(c *gin.Context) {
	organizationId, err := utils.OrgIDFromCtx(c.Request.Context(), c.Request)
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

	usecase := api.UsecasesWithCreds(c.Request).NewRuleUsecase()
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

func (api *API) GetRule(c *gin.Context) {
	ruleID := c.Param("rule_id")

	usecase := api.UsecasesWithCreds(c.Request).NewRuleUsecase()
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

func (api *API) UpdateRule(c *gin.Context) {
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

	usecase := api.UsecasesWithCreds(c.Request).NewRuleUsecase()
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

func (api *API) DeleteRule(c *gin.Context) {
	ruleID := c.Param("rule_id")

	usecase := api.UsecasesWithCreds(c.Request).NewRuleUsecase()
	err := usecase.DeleteRule(c.Request.Context(), ruleID)
	if presentError(c, err) {
		return
	}
	c.Status(http.StatusNoContent)
}
