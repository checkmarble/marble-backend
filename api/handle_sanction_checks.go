package api

import (
	"encoding/json"
	"net/http"

	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/usecases"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/cockroachdb/errors"
	"github.com/gin-gonic/gin"
)

func handleSanctionCheckDatasetFreshness(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		uc := usecasesWithCreds(ctx, uc).NewSanctionCheckUsecase()

		dataset, err := uc.CheckDatasetFreshness(ctx)
		if err != nil {
			utils.LoggerFromContext(ctx).WarnContext(ctx,
				"could not check OpenSanctions dataset freshness", "error", err.Error())

			c.JSON(http.StatusOK, dto.CreateOpenSanctionsFreshnessFallback())
			return
		}

		c.JSON(http.StatusOK, dto.AdaptSanctionCheckDataset(dataset))
	}
}

func handleSanctionCheckDatasetCatalog(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		uc := usecasesWithCreds(ctx, uc).NewSanctionCheckUsecase()

		catalog, err := uc.GetDatasetCatalog(ctx)

		if presentError(ctx, c, err) {
			return
		}

		c.JSON(http.StatusOK, dto.AdaptOpenSanctionsCatalog(catalog))
	}
}

func handleListSanctionChecks(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		decisionId := c.Query("decision_id")
		initialOnly := c.Query("initial_only") == "1"

		if decisionId == "" {
			c.Status(http.StatusBadRequest)
			return
		}

		uc := usecasesWithCreds(ctx, uc).NewSanctionCheckUsecase()
		sanctionChecks, err := uc.ListSanctionChecks(ctx, decisionId, initialOnly)

		if presentError(ctx, c, err) {
			return
		}

		c.JSON(http.StatusOK, pure_utils.Map(sanctionChecks, dto.AdaptSanctionCheckDto))
	}
}

func handleUpdateSanctionCheckMatchStatus(uc usecases.Usecases) func(c *gin.Context) {
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

func handleUploadSanctionCheckMatchFile(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		matchId := c.Param("sanctionCheckId")

		var form FileForm

		if err := c.ShouldBind(&form); err != nil {
			presentError(ctx, c, errors.Wrap(models.BadParameterError, err.Error()))
			return
		}

		creds, ok := utils.CredentialsFromCtx(ctx)

		if !ok {
			presentError(ctx, c, models.ErrUnknownUser)
			return
		}

		uc := usecasesWithCreds(ctx, uc).NewSanctionCheckUsecase()

		files, err := uc.CreateFiles(ctx, creds, matchId, form.Files)

		if presentError(ctx, c, err) {
			return
		}

		c.JSON(http.StatusCreated, pure_utils.Map(files, dto.AdaptSanctionCheckFileDto))
	}
}

func handleListSanctionCheckMatchFiles(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		sanctionCheckId := c.Param("sanctionCheckId")

		uc := usecasesWithCreds(ctx, uc).NewSanctionCheckUsecase()

		files, err := uc.ListFiles(ctx, sanctionCheckId)

		if presentError(ctx, c, err) {
			return
		}

		c.JSON(http.StatusOK, pure_utils.Map(files, dto.AdaptSanctionCheckFileDto))
	}
}

func handleDownloadSanctionCheckMatchFile(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		sanctionCheckId := c.Param("sanctionCheckId")
		fileId := c.Param("fileId")

		uc := usecasesWithCreds(ctx, uc).NewSanctionCheckUsecase()

		url, err := uc.GetFileDownloadUrl(ctx, sanctionCheckId, fileId)

		if presentError(ctx, c, err) {
			return
		}

		c.JSON(http.StatusOK, gin.H{"url": url})
	}
}

//nolint:unused
func handleCreateSanctionCheckMatchComment(uc usecases.Usecases) func(c *gin.Context) {
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

func handleRefineSanctionCheck(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		var payload dto.SanctionCheckRefineDto

		if presentError(ctx, c, c.ShouldBindJSON(&payload)) {
			return
		}

		creds, ok := utils.CredentialsFromCtx(ctx)

		if !ok {
			presentError(ctx, c, models.ErrUnknownUser)
			return
		}

		uc := usecasesWithCreds(ctx, uc).NewSanctionCheckUsecase()
		sanctionCheck, err := uc.Refine(ctx, dto.AdaptSanctionCheckRefineDto(payload), &creds.ActorIdentity.UserId)

		if presentError(ctx, c, err) {
			return
		}

		c.JSON(http.StatusOK, dto.AdaptSanctionCheckDto(sanctionCheck))
	}
}

func handleSearchSanctionCheck(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		var payload dto.SanctionCheckRefineDto

		if presentError(ctx, c, c.ShouldBindJSON(&payload)) {
			return
		}

		uc := usecasesWithCreds(ctx, uc).NewSanctionCheckUsecase()
		sanctionCheck, err := uc.Search(ctx, dto.AdaptSanctionCheckRefineDto(payload))

		if presentError(ctx, c, err) {
			return
		}

		c.JSON(http.StatusOK, pure_utils.Map(dto.AdaptSanctionCheckDto(sanctionCheck).Matches, func(
			match dto.SanctionCheckMatchDto,
		) json.RawMessage {
			return match.Payload
		}))
	}
}

func handleEnrichSanctionCheckMatch(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		matchId := c.Param("id")

		uc := usecasesWithCreds(ctx, uc).NewSanctionCheckUsecase()
		newMatch, err := uc.EnrichMatch(ctx, matchId)

		if presentError(ctx, c, err) {
			return
		}

		c.JSON(http.StatusOK, dto.AdaptSanctionCheckMatchDto(newMatch))
	}
}
