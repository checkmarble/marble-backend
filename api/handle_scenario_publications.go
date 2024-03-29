package api

import (
	"fmt"
	"net/http"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/gin-gonic/gin"

	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/utils"
)

type APIScenarioPublication struct {
	Id                  string    `json:"id"`
	Rank                int32     `json:"rank"`
	ScenarioId          string    `json:"scenarioID"`
	ScenarioIterationId string    `json:"scenarioIterationID"`
	PublicationAction   string    `json:"publicationAction"`
	CreatedAt           time.Time `json:"createdAt"`
}

func NewAPIScenarioPublication(sp models.ScenarioPublication) APIScenarioPublication {
	return APIScenarioPublication{
		Id:                  sp.Id,
		Rank:                sp.Rank,
		ScenarioId:          sp.ScenarioId,
		ScenarioIterationId: sp.ScenarioIterationId,
		PublicationAction:   sp.PublicationAction.String(),
		CreatedAt:           sp.CreatedAt,
	}
}

func (api *API) ListScenarioPublications(c *gin.Context) {
	scenarioID := c.Query("scenarioID")
	scenarioIterationID := c.Query("scenarioIterationID")

	usecase := api.UsecasesWithCreds(c.Request).NewScenarioPublicationUsecase()
	scenarioPublications, err := usecase.ListScenarioPublications(
		c.Request.Context(),
		models.ListScenarioPublicationsFilters{
			ScenarioId:          utils.PtrTo(scenarioID, &utils.PtrToOptions{OmitZero: true}),
			ScenarioIterationId: utils.PtrTo(scenarioIterationID, &utils.PtrToOptions{OmitZero: true}),
		})
	if presentError(c, err) {
		return
	}
	c.JSON(http.StatusOK, pure_utils.Map(scenarioPublications, NewAPIScenarioPublication))
}

func (api *API) CreateScenarioPublication(c *gin.Context) {
	var data dto.CreateScenarioPublicationBody
	if err := c.ShouldBindJSON(&data); err != nil {
		c.Status(http.StatusBadRequest)
		return
	}

	usecase := api.UsecasesWithCreds(c.Request).NewScenarioPublicationUsecase()
	scenarioPublications, err := usecase.ExecuteScenarioPublicationAction(
		c.Request.Context(),
		models.PublishScenarioIterationInput{
			ScenarioIterationId: data.ScenarioIterationId,
			PublicationAction:   models.PublicationActionFrom(data.PublicationAction),
		})
	if handleExpectedPublicationError(c, err) || presentError(c, err) {
		return
	}
	c.JSON(http.StatusOK, pure_utils.Map(scenarioPublications, NewAPIScenarioPublication))
}

func (api *API) GetScenarioPublication(c *gin.Context) {
	scenarioPublicationID := c.Param("publication_id")

	usecase := api.UsecasesWithCreds(c.Request).NewScenarioPublicationUsecase()
	scenarioPublication, err := usecase.GetScenarioPublication(c.Request.Context(), scenarioPublicationID)
	if presentError(c, err) {
		return
	}
	c.JSON(http.StatusOK, NewAPIScenarioPublication(scenarioPublication))
}

func (api *API) GetPublicationPreparationStatus(c *gin.Context) {
	var data struct {
		ScenarioIterationId string `form:"scenario_iteration_id" binding:"required"`
	}
	if err := c.ShouldBindQuery(&data); err != nil {
		c.Status(http.StatusBadRequest)
		return
	}

	usecase := api.UsecasesWithCreds(c.Request).NewScenarioPublicationUsecase()
	status, err := usecase.GetPublicationPreparationStatus(c.Request.Context(), data.ScenarioIterationId)
	if presentError(c, err) {
		return
	}
	c.JSON(http.StatusOK, dto.AdaptPublicationPreparationStatus(status))
}

func (api *API) StartPublicationPreparation(c *gin.Context) {
	var data struct {
		ScenarioIterationId string `json:"scenario_iteration_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&data); err != nil {
		c.Status(http.StatusBadRequest)
		return
	}

	usecase := api.UsecasesWithCreds(c.Request).NewScenarioPublicationUsecase()
	err := usecase.StartPublicationPreparation(c.Request.Context(), data.ScenarioIterationId)
	if handleExpectedPublicationError(c, err) || presentError(c, err) {
		return
	}
	c.Status(http.StatusAccepted)
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
