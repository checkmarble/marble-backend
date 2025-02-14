package api

import (
	"io"
	"net/http"

	"github.com/cockroachdb/errors"
	"github.com/gin-gonic/gin"

	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/models/ast"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/usecases"
	"github.com/checkmarble/marble-backend/utils"
)

func listScenarios(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		organizationId, err := utils.OrganizationIdFromRequest(c.Request)
		if presentError(ctx, c, err) {
			return
		}

		usecase := usecasesWithCreds(ctx, uc).NewScenarioUsecase()
		scenarios, err := usecase.ListScenarios(ctx, organizationId)
		if presentError(ctx, c, err) {
			return
		}

		scenariosDto, err := pure_utils.MapErr(scenarios, dto.AdaptScenarioDto)
		if presentError(ctx, c, err) {
			return
		}

		c.JSON(http.StatusOK, scenariosDto)
	}
}

func createScenario(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		organizationId, err := utils.OrganizationIdFromRequest(c.Request)
		if presentError(ctx, c, err) {
			return
		}

		var input dto.CreateScenarioBody
		if err := c.ShouldBindJSON(&input); err != nil {
			c.Status(http.StatusBadRequest)
			return
		}

		usecase := usecasesWithCreds(ctx, uc).NewScenarioUsecase()
		scenario, err := usecase.CreateScenario(
			ctx,
			dto.AdaptCreateScenarioInput(input, organizationId))
		if presentError(ctx, c, err) {
			return
		}

		scenarioDto, err := dto.AdaptScenarioDto(scenario)
		if presentError(ctx, c, err) {
			return
		}

		c.JSON(http.StatusOK, scenarioDto)
	}
}

func getScenario(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		id := c.Param("scenario_id")

		usecase := usecasesWithCreds(ctx, uc).NewScenarioUsecase()
		scenario, err := usecase.GetScenario(ctx, id)

		if presentError(ctx, c, err) {
			return
		}

		scenarioDto, err := dto.AdaptScenarioDto(scenario)
		if presentError(ctx, c, err) {
			return
		}

		c.JSON(http.StatusOK, scenarioDto)
	}
}

func updateScenario(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		var input dto.UpdateScenarioBody
		if err := c.ShouldBindJSON(&input); err != nil {
			c.Status(http.StatusBadRequest)
			return
		}
		scenarioId := c.Param("scenario_id")

		usecase := usecasesWithCreds(ctx, uc).NewScenarioUsecase()

		scenario, err := usecase.UpdateScenario(
			ctx,
			dto.AdaptUpdateScenarioInput(scenarioId, input))
		if presentError(ctx, c, err) {
			return
		}

		scenarioDto, err := dto.AdaptScenarioDto(scenario)
		if presentError(ctx, c, err) {
			return
		}

		c.JSON(http.StatusOK, scenarioDto)
	}
}

type PostScenarioAstValidationInputBody struct {
	Node               dto.NodeDto `json:"node" binding:"required"`
	ExpectedReturnType string      `json:"expected_return_type"`
}

func validateScenarioAst(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		var input PostScenarioAstValidationInputBody
		err := c.ShouldBindJSON(&input)
		if err != nil && err != io.EOF { //nolint:errorlint
			c.Status(http.StatusBadRequest)
			return
		}

		scenarioId := c.Param("scenario_id")

		astNode, err := dto.AdaptASTNode(input.Node)
		if err != nil {
			presentError(ctx, c, errors.Wrap(models.BadParameterError, err.Error()))
			return
		}

		expectedReturnType := make([]string, 0, 1)
		if input.ExpectedReturnType != "" {
			expectedReturnType[0] = input.ExpectedReturnType
		}

		usecase := usecasesWithCreds(ctx, uc).NewScenarioUsecase()
		astValidation, err := usecase.ValidateScenarioAst(ctx, scenarioId, &astNode, expectedReturnType...)

		if presentError(ctx, c, err) {
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"ast_validation": ast.AdaptNodeEvaluationDto(astValidation),
		})
	}
}
