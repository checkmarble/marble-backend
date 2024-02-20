package api

import (
	"errors"
	"fmt"
	"io"
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
	scenarioIterations, err := usecase.ListScenarioIterations(c.Request.Context(),
		models.GetScenarioIterationFilters{
			ScenarioId: utils.PtrTo(scenarioID, &utils.PtrToOptions{OmitZero: true}),
		})
	if presentError(c, err) {
		return
	}

	scenarioIterationsDtos := make([]dto.ScenarioIterationWithBodyDto, len(scenarioIterations))
	for i, si := range scenarioIterations {
		scenarioIterationDTO, err := dto.AdaptScenarioIterationWithBodyDto(si)
		if err != nil {
			presentError(c, err)
			return
		}
		scenarioIterationsDtos[i] = scenarioIterationDTO
	}
	c.JSON(http.StatusOK, scenarioIterationsDtos)
}

func (api *API) CreateScenarioIteration(c *gin.Context) {
	ctx := c.Request.Context()

	organizationId, err := utils.OrgIDFromCtx(ctx, c.Request)
	if presentError(c, err) {
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
			createScenarioIterationInput.Body.Rules[i], err =
				dto.AdaptCreateRuleInput(rule, organizationId)
			if presentError(c, err) {
				return
			}
		}

		if input.Body.TriggerConditionAstExpression != nil {
			trigger, err := dto.AdaptASTNode(*input.Body.TriggerConditionAstExpression)
			if err != nil {
				presentError(c, fmt.Errorf("invalid trigger: %w %w", err, models.BadParameterError))
				return
			}
			createScenarioIterationInput.Body.TriggerConditionAstExpression = &trigger
		}

	}

	usecase := api.UsecasesWithCreds(c.Request).NewScenarioIterationUsecase()
	si, err := usecase.CreateScenarioIteration(ctx, organizationId, createScenarioIterationInput)
	if presentError(c, err) {
		return
	}

	apiScenarioIterationWithBody, err := dto.AdaptScenarioIterationWithBodyDto(si)
	if presentError(c, err) {
		return
	}
	c.JSON(http.StatusOK, apiScenarioIterationWithBody)
}

func (api *API) CreateDraftFromIteration(c *gin.Context) {
	ctx := c.Request.Context()

	organizationId, err := utils.OrgIDFromCtx(ctx, c.Request)
	if presentError(c, err) {
		return
	}

	iterationID := c.Param("iteration_id")

	usecase := api.UsecasesWithCreds(c.Request).NewScenarioIterationUsecase()
	si, err := usecase.CreateDraftFromScenarioIteration(ctx, organizationId, iterationID)
	if presentError(c, err) {
		return
	}

	apiScenarioIterationWithBody, err := dto.AdaptScenarioIterationWithBodyDto(si)
	if presentError(c, err) {
		return
	}
	c.JSON(http.StatusOK, apiScenarioIterationWithBody)
}

func (api *API) GetScenarioIteration(c *gin.Context) {
	iterationID := c.Param("iteration_id")

	usecase := api.UsecasesWithCreds(c.Request).NewScenarioIterationUsecase()
	si, err := usecase.GetScenarioIteration(c.Request.Context(), iterationID)
	if presentError(c, err) {
		return
	}

	scenarioIterationDto, err := dto.AdaptScenarioIterationWithBodyDto(si)
	if presentError(c, err) {
		return
	}
	c.JSON(http.StatusOK, scenarioIterationDto)
}

func (api *API) UpdateScenarioIteration(c *gin.Context) {
	organizationId, err := utils.OrganizationIdFromRequest(c.Request)
	if presentError(c, err) {
		return
	}

	iterationID := c.Param("iteration_id")
	var data dto.UpdateScenarioIterationBody
	if err := c.ShouldBindJSON(&data); err != nil {
		c.Status(http.StatusBadRequest)
		return
	}

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
			presentError(c, fmt.Errorf("invalid trigger: %w %w", err, models.BadParameterError))
			return
		}
		updateScenarioIterationInput.Body.TriggerConditionAstExpression = &trigger
	}

	usecase := api.UsecasesWithCreds(c.Request).NewScenarioIterationUsecase()
	updatedSI, err := usecase.UpdateScenarioIteration(c.Request.Context(), organizationId, updateScenarioIterationInput)
	if handleExpectedIterationError(c, err) || presentError(c, err) {
		return
	}

	iteration, err := dto.AdaptScenarioIterationWithBodyDto(updatedSI)
	if presentError(c, err) {
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
	scenarioValidation, err := usecase.ValidateScenarioIteration(c.Request.Context(),
		scenarioIterationID, triggerOrRule, input.RuleId)

	if presentError(c, err) {
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"scenario_validation": dto.AdaptScenarioValidationDto(scenarioValidation),
	})
}

func (api *API) CommitScenarioIterationVersion(c *gin.Context) {
	scenarioIterationID := c.Param("iteration_id")

	usecase := api.UsecasesWithCreds(c.Request).NewScenarioIterationUsecase()
	iteration, err := usecase.CommitScenarioIterationVersion(c.Request.Context(), scenarioIterationID)
	if handleExpectedIterationError(c, err) || presentError(c, err) {
		return
	}

	iterationDto, err := dto.AdaptScenarioIterationWithBodyDto(iteration)
	if presentError(c, err) {
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"iteration": iterationDto,
	})
}

func handleExpectedIterationError(c *gin.Context, err error) bool {
	if err == nil {
		return false
	}
	logger := utils.LoggerFromContext(c.Request.Context())
	logger.InfoContext(c.Request.Context(), fmt.Sprintf("error: %v", err))
	if errors.Is(err, models.ErrScenarioIterationNotDraft) {
		c.JSON(http.StatusBadRequest, dto.APIErrorResponse{
			Message:   "Only a draft iteration can be committed or edited",
			ErrorCode: dto.CanOnlyEditDraft,
		})
		return true
	}

	return false
}
