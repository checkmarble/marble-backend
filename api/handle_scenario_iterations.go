package api

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/models/ast"
	"github.com/checkmarble/marble-backend/utils"
)

func (api *API) ListScenarioIterations(c *gin.Context) {
	scenarioID := c.Query("scenarioId")

	usecase := api.UsecasesWithCreds(c.Request).NewScenarioIterationUsecase()
	scenarioIterations, err := usecase.ListScenarioIterations(models.GetScenarioIterationFilters{
		ScenarioId: utils.PtrTo(scenarioID, &utils.PtrToOptions{OmitZero: true}),
	})
	if presentError(c.Writer, c.Request, err) {
		return
	}

	scenarioIterationsDtos := make([]dto.ScenarioIterationWithBodyDto, len(scenarioIterations))
	for i, si := range scenarioIterations {
		scenarioIterationDTO, err := dto.AdaptScenarioIterationWithBodyDto(si)
		if err != nil {
			presentError(c.Writer, c.Request, err)
			return
		}
		scenarioIterationsDtos[i] = scenarioIterationDTO
	}
	c.JSON(http.StatusOK, scenarioIterationsDtos)
}

func (api *API) CreateScenarioIteration(c *gin.Context) {
	ctx := c.Request.Context()

	organizationId, err := utils.OrgIDFromCtx(ctx, c.Request)
	if presentError(c.Writer, c.Request, err) {
		return
	}

	var input dto.CreateScenarioIterationBody
	if err := c.ShouldBindJSON(&input); err != nil {
		c.Status(http.StatusBadRequest)
		return
	}

	createScenarioIterationInput := models.CreateScenarioIterationInput{
		ScenarioId: input.ScenarioId,
	}

	if input.Body != nil {
		createScenarioIterationInput.Body = &models.CreateScenarioIterationBody{
			ScoreReviewThreshold: input.Body.ScoreReviewThreshold,
			ScoreRejectThreshold: input.Body.ScoreRejectThreshold,
			BatchTriggerSQL:      input.Body.BatchTriggerSQL,
			Schedule:             input.Body.Schedule,
			Rules:                make([]models.CreateRuleInput, len(input.Body.Rules)),
		}

		for i, rule := range input.Body.Rules {
			createScenarioIterationInput.Body.Rules[i], err = dto.AdaptCreateRuleInput(rule, organizationId)
			if presentError(c.Writer, c.Request, err) {
				return
			}
		}

		if input.Body.TriggerConditionAstExpression != nil {
			trigger, err := dto.AdaptASTNode(*input.Body.TriggerConditionAstExpression)
			if err != nil {
				presentError(c.Writer, c.Request, fmt.Errorf("invalid trigger: %w %w", err, models.BadParameterError))
				return
			}
			createScenarioIterationInput.Body.TriggerConditionAstExpression = &trigger
		}

	}

	usecase := api.UsecasesWithCreds(c.Request).NewScenarioIterationUsecase()
	si, err := usecase.CreateScenarioIteration(ctx, organizationId, createScenarioIterationInput)
	if presentError(c.Writer, c.Request, err) {
		return
	}

	apiScenarioIterationWithBody, err := dto.AdaptScenarioIterationWithBodyDto(si)
	if presentError(c.Writer, c.Request, err) {
		return
	}
	c.JSON(http.StatusOK, apiScenarioIterationWithBody)
}

func (api *API) CreateDraftFromIteration(c *gin.Context) {
	ctx := c.Request.Context()

	organizationId, err := utils.OrgIDFromCtx(ctx, c.Request)
	if presentError(c.Writer, c.Request, err) {
		return
	}

	iterationID := c.Param("iteration_id")

	usecase := api.UsecasesWithCreds(c.Request).NewScenarioIterationUsecase()
	si, err := usecase.CreateDraftFromScenarioIteration(ctx, organizationId, iterationID)
	if presentError(c.Writer, c.Request, err) {
		return
	}

	apiScenarioIterationWithBody, err := dto.AdaptScenarioIterationWithBodyDto(si)
	if presentError(c.Writer, c.Request, err) {
		return
	}
	c.JSON(http.StatusOK, apiScenarioIterationWithBody)
}

func (api *API) GetScenarioIteration(c *gin.Context) {
	iterationID := c.Param("iteration_id")

	usecase := api.UsecasesWithCreds(c.Request).NewScenarioIterationUsecase()
	si, err := usecase.GetScenarioIteration(iterationID)
	if presentError(c.Writer, c.Request, err) {
		return
	}

	scenarioIterationDto, err := dto.AdaptScenarioIterationWithBodyDto(si)
	if presentError(c.Writer, c.Request, err) {
		return
	}
	c.JSON(http.StatusOK, scenarioIterationDto)
}

func (api *API) UpdateScenarioIteration(c *gin.Context) {
	ctx := c.Request.Context()
	logger := utils.LoggerFromContext(ctx)

	organizationId, err := utils.OrganizationIdFromRequest(c.Request)
	if presentError(c.Writer, c.Request, err) {
		return
	}

	iterationID := c.Param("iteration_id")
	var data dto.UpdateScenarioIterationBody
	if err := c.ShouldBindJSON(&data); err != nil {
		c.Status(http.StatusBadRequest)
		return
	}

	logger = logger.With(slog.String("scenarioIterationId", iterationID), slog.String("organizationId", organizationId))

	updateScenarioIterationInput := models.UpdateScenarioIterationInput{
		Id: iterationID,
		Body: &models.UpdateScenarioIterationBody{
			ScoreReviewThreshold: data.Body.ScoreReviewThreshold,
			ScoreRejectThreshold: data.Body.ScoreRejectThreshold,
			Schedule:             data.Body.Schedule,
			BatchTriggerSQL:      data.Body.BatchTriggerSQL,
		},
	}

	if data.Body.TriggerConditionAstExpression != nil {
		trigger, err := dto.AdaptASTNode(*data.Body.TriggerConditionAstExpression)
		if err != nil {
			presentError(c.Writer, c.Request, fmt.Errorf("invalid trigger: %w %w", err, models.BadParameterError))
			return
		}
		updateScenarioIterationInput.Body.TriggerConditionAstExpression = &trigger
	}

	usecase := api.UsecasesWithCreds(c.Request).NewScenarioIterationUsecase()
	updatedSI, err := usecase.UpdateScenarioIteration(ctx, organizationId, updateScenarioIterationInput)
	if errors.Is(err, models.ErrScenarioIterationNotDraft) {
		logger.WarnContext(ctx, "Cannot update scenario iteration that is not in draft state: \n"+err.Error())
		http.Error(c.Writer, "", http.StatusForbidden)
		return
	}

	if presentError(c.Writer, c.Request, err) {
		return
	}

	iteration, err := dto.AdaptScenarioIterationWithBodyDto(updatedSI)
	if presentError(c.Writer, c.Request, err) {
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"iteration": iteration,
	})
}

type PostScenarioValidationInputBody struct {
	TriggerOrRule *dto.NodeDto `json:"trigger_or_rule"`
	RuleId        *string      `json:"rule_id"`
}

func (api *API) ValidateScenarioIteration(c *gin.Context) {
	var input PostScenarioValidationInputBody
	err := c.ShouldBindJSON(&input)
	if err != nil && err != io.EOF {
		c.Status(http.StatusBadRequest)
		return
	}

	scenarioIterationID := c.Param("iteration_id")

	var triggerOrRule *ast.Node
	if input.TriggerOrRule != nil {
		node, err := dto.AdaptASTNode(*input.TriggerOrRule)
		if err != nil {
			c.Status(http.StatusInternalServerError)
			return
		}
		triggerOrRule = &node
	}

	usecase := api.UsecasesWithCreds(c.Request).NewScenarioIterationUsecase()
	scenarioValidation, err := usecase.ValidateScenarioIteration(scenarioIterationID, triggerOrRule, input.RuleId)

	if presentError(c.Writer, c.Request, err) {
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"scenario_validation": dto.AdaptScenarioValidationDto(scenarioValidation),
	})
}
