package api

import (
	"net/http"

	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/usecases"
	"github.com/gin-gonic/gin"
)

func handleListSanctionChecks(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		decisionId := c.Query("decision_id")

		if decisionId == "" {
			c.Status(http.StatusBadRequest)
			return
		}

		uc := usecasesWithCreds(ctx, uc).NewSanctionCheckUsecase()
		sanctionChecks, err := uc.ListSanctionChecks(ctx, decisionId)

		if presentError(ctx, c, err) {
			return
		}

		sanctionCheckJson := pure_utils.Map(sanctionChecks, dto.AdaptSanctionCheckDto)

		c.JSON(http.StatusOK, sanctionCheckJson)
	}
}

func handleUpdateSanctionCheckMatchStatus(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		matchId := c.Param("id")

		var update dto.SanctionCheckMatchUpdateDto

		if presentError(ctx, c, c.ShouldBindJSON(&update)) ||
			presentError(ctx, c, update.Validate()) {
			return
		}

		uc := usecasesWithCreds(ctx, uc).NewSanctionCheckUsecase()

		match, err := uc.UpdateMatchStatus(ctx, matchId, models.SanctionCheckMatchUpdate{
			Status: update.Status,
		})

		if presentError(ctx, c, err) {
			return
		}

		c.JSON(http.StatusOK, dto.AdaptSanctionCheckMatchDto(match))
	}
}

func handleCreateSanctionCheckMatchComment(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		matchId := c.Param("id")

		var payload dto.SanctionCheckMatchCommentDto

		if presentError(ctx, c, c.ShouldBindJSON(&payload)) {
			return
		}

		uc := usecasesWithCreds(ctx, uc).NewSanctionCheckUsecase()

		comment, err := uc.MatchAddComment(ctx, matchId, models.SanctionCheckMatchComment{
			MatchId: matchId,
			Comment: payload.Comment,
		})

		if presentError(ctx, c, err) {
			return
		}

		c.JSON(http.StatusCreated, dto.AdaptSanctionCheckMatchCommentDto(comment))
	}
}
