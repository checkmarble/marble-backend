package api

import (
	"net/http"

	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/usecases"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/gin-gonic/gin"
)

func handleSanctionCheckDataset(uc usecases.Usecaser) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		uc := uc.WithCreds(ctx).NewSanctionCheckUsecase()

		dataset, err := uc.CheckDataset(ctx)

		if presentError(ctx, c, err) {
			return
		}

		c.JSON(http.StatusOK, dataset)
	}
}

func handleListSanctionChecks(uc usecases.Usecaser) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		decisionId := c.Query("decision_id")

		if decisionId == "" {
			c.Status(http.StatusBadRequest)
			return
		}

		uc := uc.WithCreds(ctx).NewSanctionCheckUsecase()
		sanctionChecks, err := uc.ListSanctionChecks(ctx, decisionId)

		if presentError(ctx, c, err) {
			return
		}

		sanctionCheckJson := pure_utils.Map(sanctionChecks, dto.AdaptSanctionCheckDto)

		c.JSON(http.StatusOK, sanctionCheckJson)
	}
}

func handleUpdateSanctionCheckMatchStatus(uc usecases.Usecaser) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		matchId := c.Param("id")

		var payload dto.SanctionCheckMatchUpdateDto

		if presentError(ctx, c, c.ShouldBindJSON(&payload)) {
			return
		}

		creds, ok := utils.CredentialsFromCtx(ctx)

		if !ok {
			presentError(ctx, c, models.ErrUnknownUser)
			return
		}

		update, err := dto.AdaptSanctionCheckMatchUpdateInputDto(matchId, creds.ActorIdentity.UserId, payload)

		if presentError(ctx, c, err) {
			return
		}

		uc := usecasesWithCreds(ctx, uc).NewSanctionCheckUsecase()

		match, err := uc.UpdateMatchStatus(ctx, update)

		if presentError(ctx, c, err) {
			return
		}

		c.JSON(http.StatusOK, dto.AdaptSanctionCheckMatchDto(match))
	}
}

func handleListSanctionCheckMatchComments(uc usecases.Usecaser) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		matchId := c.Param("id")

		uc := usecasesWithCreds(ctx, uc).NewSanctionCheckUsecase()

		comments, err := uc.MatchListComments(ctx, matchId)

		if presentError(ctx, c, err) {
			return
		}

		c.JSON(http.StatusOK, pure_utils.Map(comments, dto.AdaptSanctionCheckMatchCommentDto))
	}
}

func handleCreateSanctionCheckMatchComment(uc usecases.Usecaser) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		matchId := c.Param("id")

		var payload dto.SanctionCheckMatchCommentDto

		if presentError(ctx, c, c.ShouldBindJSON(&payload)) {
			return
		}

		creds, ok := utils.CredentialsFromCtx(ctx)

		if !ok {
			presentError(ctx, c, models.ErrUnknownUser)
			return
		}

		uc := usecasesWithCreds(ctx, uc).NewSanctionCheckUsecase()
		comment, err := dto.AdaptSanctionCheckMatchCommentInputDto(matchId, creds.ActorIdentity.UserId, payload)

		if presentError(ctx, c, err) {
			return
		}

		comment, err = uc.MatchAddComment(ctx, matchId, comment)

		if presentError(ctx, c, err) {
			return
		}

		c.JSON(http.StatusCreated, dto.AdaptSanctionCheckMatchCommentDto(comment))
	}
}
