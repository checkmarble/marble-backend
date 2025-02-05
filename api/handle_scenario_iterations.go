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
	"github.com/checkmarble/marble-backend/usecases"
	"github.com/checkmarble/marble-backend/utils"
)

func handleListScenarioIterations(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		scenarioId := c.Query("scenario_id")
		organizationId, err := utils.OrganizationIdFromRequest(c.Request)
		if presentError(ctx, c, err) {
			return
		}

		usecase := usecasesWithCreds(ctx, uc).NewScenarioIterationUsecase()
		scenarioIterations, err := usecase.ListScenarioIterations(
			ctx,
			organizationId,
			models.GetScenarioIterationFilters{
				ScenarioId: utils.PtrTo(scenarioId, &utils.PtrToOptions{OmitZero: true}),
			})
		if presentError(ctx, c, err) {
			return
		}

		scenarioIterationsDtos := make([]dto.ScenarioIterationWithBodyDto, len(scenarioIterations))
		for i, si := range scenarioIterations {
			scenarioIterationDTO, err := dto.AdaptScenarioIterationWithBodyDto(si)
			if err != nil {
				presentError(ctx, c, err)
				return
			}
			scenarioIterationsDtos[i] = scenarioIterationDTO
		}
		c.JSON(http.StatusOK, scenarioIterationsDtos)
	}
}

func handleCreateScenarioIteration(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		organizationId, err := utils.OrganizationIdFromRequest(c.Request)
		if presentError(ctx, c, err) {
			return
		}

		var input dto.CreateScenarioIterationBody
		if err := c.ShouldBindJSON(&input); err != nil {
			c.Status(http.StatusBadRequest)
			return
		}

		createScenarioIterationInput, err := dto.AdaptCreateScenarioIterationInput(input, organizationId)
		if presentError(ctx, c, err) {
			return
		}

		usecase := usecasesWithCreds(ctx, uc).NewScenarioIterationUsecase()
		si, err := usecase.CreateScenarioIteration(ctx, organizationId, createScenarioIterationInput)
		if presentError(ctx, c, err) {
			return
		}

		apiScenarioIterationWithBody, err := dto.AdaptScenarioIterationWithBodyDto(si)
		if presentError(ctx, c, err) {
			return
		}
		c.JSON(http.StatusOK, apiScenarioIterationWithBody)
	}
}

func handleConfigureSanctionCheck(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		iterationId := c.Param("iteration_id")
		ctx := c.Request.Context()

		var input dto.SanctionCheckConfig

		if err := c.ShouldBindJSON(&input); err != nil {
			c.Status(http.StatusBadRequest)
			return
		}

		config, err := dto.AdaptSanctionCheckConfigInputDto(input)

		if presentError(ctx, c, err) {
			c.Status(http.StatusBadRequest)
			return
		}

		uc := usecasesWithCreds(ctx, uc).NewSanctionCheckUsecase()

		scc, err := uc.ConfigureSanctionCheck(ctx, iterationId, config)

		if presentError(ctx, c, err) {
			return
		}

		output, err := dto.AdaptSanctionCheckConfig(scc)

		if presentError(ctx, c, err) {
			return
		}

		c.JSON(http.StatusOK, output)
	}
}

func handleCreateDraftFromIteration(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		organizationId, err := utils.OrganizationIdFromRequest(c.Request)
		if presentError(ctx, c, err) {
			return
		}

		iterationID := c.Param("iteration_id")

		usecase := usecasesWithCreds(ctx, uc).NewScenarioIterationUsecase()
		si, err := usecase.CreateDraftFromScenarioIteration(ctx, organizationId, iterationID)
		if presentError(ctx, c, err) {
			return
		}

		apiScenarioIterationWithBody, err := dto.AdaptScenarioIterationWithBodyDto(si)
		if presentError(ctx, c, err) {
			return
		}
		c.JSON(http.StatusOK, apiScenarioIterationWithBody)
	}
}

func handleGetScenarioIteration(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		iterationID := c.Param("iteration_id")

		usecase := usecasesWithCreds(ctx, uc).NewScenarioIterationUsecase()
		si, err := usecase.GetScenarioIteration(ctx, iterationID)
		if presentError(ctx, c, err) {
			return
		}

		scenarioIterationDto, err := dto.AdaptScenarioIterationWithBodyDto(si)
		if presentError(ctx, c, err) {
			return
		}
		c.JSON(http.StatusOK, scenarioIterationDto)
	}
}

func handleUpdateScenarioIteration(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		organizationId, err := utils.OrganizationIdFromRequest(c.Request)
		if presentError(ctx, c, err) {
			return
		}

		iterationID := c.Param("iteration_id")
		var data dto.UpdateScenarioIterationBody
		if err := c.ShouldBindJSON(&data); err != nil {
			c.Status(http.StatusBadRequest)
			return
		}

		updateScenarioIterationInput, err := dto.AdaptUpdateScenarioIterationInput(data, iterationID)
		if presentError(ctx, c, err) {
			return
		}

		usecase := usecasesWithCreds(ctx, uc).NewScenarioIterationUsecase()
		updatedSI, err := usecase.UpdateScenarioIteration(ctx,
			organizationId, updateScenarioIterationInput)
		if handleExpectedIterationError(c, err) || presentError(ctx, c, err) {
			return
		}

		iteration, err := dto.AdaptScenarioIterationWithBodyDto(updatedSI)
		if presentError(ctx, c, err) {
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"iteration": iteration,
		})
	}
}

type PostScenarioValidationInputBody struct {
	TriggerOrRule *dto.NodeDto `json:"trigger_or_rule"`
	RuleId        *string      `json:"rule_id"`
}

func handleValidateScenarioIteration(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		var input PostScenarioValidationInputBody
		err := c.ShouldBindJSON(&input)
		if err != nil && err != io.EOF { //nolint:errorlint
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

		usecase := usecasesWithCreds(ctx, uc).NewScenarioIterationUsecase()
		scenarioValidation, err := usecase.ValidateScenarioIteration(ctx,
			scenarioIterationID, triggerOrRule, input.RuleId)

		if presentError(ctx, c, err) {
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"scenario_validation": dto.AdaptScenarioValidationDto(scenarioValidation),
		})
	}
}

func handleCommitScenarioIterationVersion(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		scenarioIterationID := c.Param("iteration_id")

		usecase := usecasesWithCreds(ctx, uc).NewScenarioIterationUsecase()
		iteration, err := usecase.CommitScenarioIterationVersion(ctx, scenarioIterationID)
		if handleExpectedIterationError(c, err) || presentError(ctx, c, err) {
			return
		}

		iterationDto, err := dto.AdaptScenarioIterationWithBodyDto(iteration)
		if presentError(ctx, c, err) {
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"iteration": iterationDto,
		})
	}
}

func handleExpectedIterationError(c *gin.Context, err error) bool {
	ctx := c.Request.Context()
	if err == nil {
		return false
	}
	logger := utils.LoggerFromContext(ctx)
	logger.InfoContext(ctx, fmt.Sprintf("error: %v", err))
	if errors.Is(err, models.ErrScenarioIterationNotDraft) {
		c.JSON(http.StatusBadRequest, dto.APIErrorResponse{
			Message:   "Only a draft iteration can be committed or edited",
			ErrorCode: dto.CanOnlyEditDraft,
		})
		return true
	}

	return false
}
