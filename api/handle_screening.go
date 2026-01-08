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

func handleScreeningDatasetFreshness(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		uc := usecasesWithCreds(ctx, uc).NewScreeningUsecase()

		dataset, err := uc.CheckDatasetFreshness(ctx)
		if err != nil {
			utils.LoggerFromContext(ctx).WarnContext(ctx,
				"could not check OpenSanctions dataset freshness", "error", err.Error())

			c.JSON(http.StatusOK, dto.CreateOpenSanctionsFreshnessFallback())
			return
		}

		c.JSON(http.StatusOK, dto.AdaptScreeningDataset(dataset))
	}
}

func handleScreeningDatasetCatalog(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		uc := usecasesWithCreds(ctx, uc).NewScreeningUsecase()

		catalog, err := uc.GetDatasetCatalog(ctx)

		if presentError(ctx, c, err) {
			return
		}

		c.JSON(http.StatusOK, dto.AdaptOpenSanctionsCatalog(catalog))
	}
}

func handleListScreenings(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		decisionId := c.Query("decision_id")
		initialOnly := c.Query("initial_only") == "1"

		if decisionId == "" {
			c.Status(http.StatusBadRequest)
			return
		}

		uc := usecasesWithCreds(ctx, uc).NewScreeningUsecase()
		screenings, err := uc.ListScreenings(ctx, decisionId, initialOnly)

		if presentError(ctx, c, err) {
			return
		}

		c.JSON(http.StatusOK, pure_utils.Map(screenings, dto.AdaptScreeningDto))
	}
}

func handleUpdateScreeningMatchStatus(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		matchId := c.Param("id")

		var payload dto.ScreeningMatchUpdateDto

		if presentError(ctx, c, c.ShouldBindJSON(&payload)) {
			return
		}

		creds, ok := utils.CredentialsFromCtx(ctx)

		if !ok {
			presentError(ctx, c, models.ErrUnknownUser)
			return
		}

		update, err := dto.AdaptScreeningMatchUpdateInputDto(matchId, creds.ActorIdentity.UserId, payload)

		if presentError(ctx, c, err) {
			return
		}

		uc := usecasesWithCreds(ctx, uc).NewScreeningUsecase()

		match, err := uc.UpdateMatchStatus(ctx, update)

		if presentError(ctx, c, err) {
			return
		}

		c.JSON(http.StatusOK, dto.AdaptScreeningMatchDto(match))
	}
}

func handleUploadScreeningMatchFile(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		screeningId := c.Param("screeningId")

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

		uc := usecasesWithCreds(ctx, uc).NewScreeningUsecase()

		files, err := uc.CreateFiles(ctx, creds, screeningId, form.Files)

		if presentError(ctx, c, err) {
			return
		}

		c.JSON(http.StatusCreated, pure_utils.Map(files, dto.AdaptScreeningFileDto))
	}
}

func handleListScreeningMatchFiles(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		screeningId := c.Param("screeningId")

		uc := usecasesWithCreds(ctx, uc).NewScreeningUsecase()

		files, err := uc.ListFiles(ctx, screeningId)

		if presentError(ctx, c, err) {
			return
		}

		c.JSON(http.StatusOK, pure_utils.Map(files, dto.AdaptScreeningFileDto))
	}
}

func handleDownloadScreeningMatchFile(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		screeningId := c.Param("screeningId")
		fileId := c.Param("fileId")

		uc := usecasesWithCreds(ctx, uc).NewScreeningUsecase()

		url, err := uc.GetFileDownloadUrl(ctx, screeningId, fileId)

		if presentError(ctx, c, err) {
			return
		}

		c.JSON(http.StatusOK, gin.H{"url": url})
	}
}

//nolint:unused
func handleCreateScreeningMatchComment(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		matchId := c.Param("id")

		var payload dto.ScreeningMatchCommentDto

		if presentError(ctx, c, c.ShouldBindJSON(&payload)) {
			return
		}

		creds, ok := utils.CredentialsFromCtx(ctx)

		if !ok {
			presentError(ctx, c, models.ErrUnknownUser)
			return
		}

		uc := usecasesWithCreds(ctx, uc).NewScreeningUsecase()
		comment, err := dto.AdaptScreeningMatchCommentInputDto(matchId, creds.ActorIdentity.UserId, payload)

		if presentError(ctx, c, err) {
			return
		}

		comment, err = uc.MatchAddComment(ctx, matchId, comment)

		if presentError(ctx, c, err) {
			return
		}

		c.JSON(http.StatusCreated, dto.AdaptScreeningMatchCommentDto(comment))
	}
}

func handleRefineScreening(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		var payload dto.ScreeningRefineDto

		if presentError(ctx, c, c.ShouldBindJSON(&payload)) {
			return
		}

		creds, ok := utils.CredentialsFromCtx(ctx)

		if !ok {
			presentError(ctx, c, models.ErrUnknownUser)
			return
		}

		uc := usecasesWithCreds(ctx, uc).NewScreeningUsecase()
		screening, err := uc.Refine(ctx, dto.AdaptScreeningRefineDto(payload), &creds.ActorIdentity.UserId)

		if presentError(ctx, c, err) {
			return
		}

		c.JSON(http.StatusOK, dto.AdaptScreeningDto(screening))
	}
}

func handleSearchScreening(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		var payload dto.ScreeningRefineDto

		if presentError(ctx, c, c.ShouldBindJSON(&payload)) {
			return
		}

		uc := usecasesWithCreds(ctx, uc).NewScreeningUsecase()
		screening, err := uc.Search(ctx, dto.AdaptScreeningRefineDto(payload))

		if presentError(ctx, c, err) {
			return
		}

		c.JSON(http.StatusOK, pure_utils.Map(dto.AdaptScreeningDto(screening).Matches, func(
			match dto.ScreeningMatchDto,
		) json.RawMessage {
			return match.Payload
		}))
	}
}

func handleEnrichScreeningMatch(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		matchId := c.Param("id")

		uc := usecasesWithCreds(ctx, uc).NewScreeningUsecase()
		newMatch, err := uc.EnrichMatch(ctx, matchId)

		if presentError(ctx, c, err) {
			return
		}

		c.JSON(http.StatusOK, dto.AdaptScreeningMatchDto(newMatch))
	}
}

func handleFreeformSearch(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		orgId, err := utils.OrganizationIdFromRequest(c.Request)
		if presentError(ctx, c, err) {
			return
		}

		var payload dto.ScreeningFreeformDto

		if presentError(ctx, c, c.ShouldBindJSON(&payload)) {
			return
		}

		req := models.ScreeningRefineRequest{
			Type:  payload.Query.Type(),
			Query: dto.AdaptRefineQueryDto(payload.Query),
		}

		scc := models.ScreeningConfig{
			Datasets:  payload.Datasets,
			Threshold: payload.Threshold,
		}

		uc := usecasesWithCreds(ctx, uc).NewScreeningUsecase()

		matches, err := uc.FreeformSearch(ctx, orgId, scc, req)
		if presentError(ctx, c, err) {
			return
		}

		c.JSON(http.StatusOK, pure_utils.Map(matches.Matches, dto.AdaptScreeningMatchDto))
	}
}
