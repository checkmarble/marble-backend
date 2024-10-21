package api

import (
	"fmt"
	"net/http"

	"github.com/cockroachdb/errors"
	"github.com/gin-gonic/gin"

	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/usecases"
	"github.com/checkmarble/marble-backend/utils"
)

func handleListScenarioPublications(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		organizationId, err := utils.OrganizationIdFromRequest(c.Request)
		if presentError(ctx, c, err) {
			return
		}

		scenarioID := c.Query("scenario_id")
		scenarioIterationID := c.Query("scenario_iteration_id")

		usecase := usecasesWithCreds(ctx, uc).NewScenarioPublicationUsecase()
		scenarioPublications, err := usecase.ListScenarioPublications(
			c.Request.Context(),
			organizationId,
			models.ListScenarioPublicationsFilters{
				ScenarioId:          utils.PtrTo(scenarioID, &utils.PtrToOptions{OmitZero: true}),
				ScenarioIterationId: utils.PtrTo(scenarioIterationID, &utils.PtrToOptions{OmitZero: true}),
			})
		if presentError(ctx, c, err) {
			return
		}
		c.JSON(http.StatusOK, pure_utils.Map(scenarioPublications, dto.AdaptScenarioPublicationDto))
	}
}

func handleCreateScenarioPublication(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		organizationId, err := utils.OrganizationIdFromRequest(c.Request)
		if presentError(ctx, c, err) {
			return
		}

		var data dto.CreateScenarioPublicationBody
		if err := c.ShouldBindJSON(&data); err != nil {
			c.Status(http.StatusBadRequest)
			return
		}

		usecase := usecasesWithCreds(ctx, uc).NewScenarioPublicationUsecase()
		scenarioPublications, err := usecase.ExecuteScenarioPublicationAction(
			c.Request.Context(),
			organizationId,
			dto.AdaptCreateScenarioPublicationBody(data))
		if handleExpectedPublicationError(c, err) || presentError(ctx, c, err) {
			return
		}
		c.JSON(http.StatusOK, pure_utils.Map(scenarioPublications, dto.AdaptScenarioPublicationDto))
	}
}

func handleGetScenarioPublication(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		scenarioPublicationID := c.Param("publication_id")

		usecase := usecasesWithCreds(ctx, uc).NewScenarioPublicationUsecase()
		scenarioPublication, err := usecase.GetScenarioPublication(c.Request.Context(), scenarioPublicationID)
		if presentError(ctx, c, err) {
			return
		}
		c.JSON(http.StatusOK, dto.AdaptScenarioPublicationDto(scenarioPublication))
	}
}

func handleGetPublicationPreparationStatus(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		organizationId, err := utils.OrganizationIdFromRequest(c.Request)
		if presentError(ctx, c, err) {
			return
		}

		var data struct {
			ScenarioIterationId string `form:"scenario_iteration_id" binding:"required"`
		}
		if err := c.ShouldBindQuery(&data); err != nil {
			c.Status(http.StatusBadRequest)
			return
		}

		usecase := usecasesWithCreds(ctx, uc).NewScenarioPublicationUsecase()
		status, err := usecase.GetPublicationPreparationStatus(
			c.Request.Context(),
			organizationId,
			data.ScenarioIterationId)
		if presentError(ctx, c, err) {
			return
		}
		c.JSON(http.StatusOK, dto.AdaptPublicationPreparationStatus(status))
	}
}

func handleStartPublicationPreparation(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		organizationId, err := utils.OrganizationIdFromRequest(c.Request)
		if presentError(ctx, c, err) {
			return
		}

		var data struct {
			ScenarioIterationId string `json:"scenario_iteration_id" binding:"required"`
		}
		if err := c.ShouldBindJSON(&data); err != nil {
			c.Status(http.StatusBadRequest)
			return
		}

		usecase := usecasesWithCreds(ctx, uc).NewScenarioPublicationUsecase()
		err = usecase.StartPublicationPreparation(c.Request.Context(), organizationId, data.ScenarioIterationId)
		if handleExpectedPublicationError(c, err) || presentError(ctx, c, err) {
			return
		}
		c.Status(http.StatusAccepted)
	}
}

func handleExpectedPublicationError(c *gin.Context, err error) bool {
	if err == nil {
		return false
	}
	logger := utils.LoggerFromContext(c.Request.Context())
	logger.InfoContext(c.Request.Context(), fmt.Sprintf("error: %v", err))

	if errors.Is(err, models.ErrScenarioIterationIsDraft) {
		c.JSON(http.StatusBadRequest, dto.APIErrorResponse{
			Message:   "You cannot activate a draft iteration",
			ErrorCode: dto.CannotPublishDraft,
		})
		return true
	} else if errors.Is(err, models.ErrScenarioIterationRequiresPreparation) {
		c.JSON(http.StatusBadRequest, dto.APIErrorResponse{
			Message:   "You cannot activate the iteration: requires data preparation to be run",
			ErrorCode: dto.CannotPublishRequiresPreparation,
		})
		return true
	} else if errors.Is(err, models.ErrScenarioIterationNotValid) {
		c.JSON(http.StatusBadRequest, dto.APIErrorResponse{
			Message:   "You cannot activate an invalid iteration",
			ErrorCode: dto.ScenarioIterationInvalid,
		})
		return true
	} else if errors.Is(err, models.ErrDataPreparationServiceUnavailable) {
		c.JSON(http.StatusConflict, dto.APIErrorResponse{
			Message:   "Data preparation service is currently busy",
			ErrorCode: dto.DataPreparationServiceUnavailable,
		})
		return true
	}
	return false
}
