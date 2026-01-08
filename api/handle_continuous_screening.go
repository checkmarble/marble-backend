package api

import (
	"net/http"

	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/usecases"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/cockroachdb/errors"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func handleGetContinuousScreeningConfig(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		stableIdString := c.Param("stable_id")
		stableId, err := uuid.Parse(stableIdString)
		if err != nil {
			presentError(ctx, c, errors.Wrap(models.BadParameterError, err.Error()))
			return
		}

		uc := usecasesWithCreds(ctx, uc).NewContinuousScreeningUsecase()
		continuousScreeningConfig, err := uc.GetContinuousScreeningConfigByStableId(ctx, stableId)
		if presentError(ctx, c, err) {
			return
		}

		c.JSON(http.StatusOK, dto.AdaptContinuousScreeningConfigDto(continuousScreeningConfig))
	}
}

func handleListContinuousScreeningConfigs(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		organizationId, err := utils.OrganizationIdFromRequest(c.Request)
		if presentError(ctx, c, err) {
			return
		}

		uc := usecasesWithCreds(ctx, uc).NewContinuousScreeningUsecase()
		continuousScreeningConfigs, err := uc.GetContinuousScreeningConfigsByOrgId(ctx, organizationId)
		if presentError(ctx, c, err) {
			return
		}

		c.JSON(http.StatusOK, pure_utils.Map(continuousScreeningConfigs, dto.AdaptContinuousScreeningConfigDto))
	}
}

func handleCreateContinuousScreeningConfig(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		organizationId, err := utils.OrganizationIdFromRequest(c.Request)
		if presentError(ctx, c, err) {
			return
		}

		var input dto.CreateContinuousScreeningConfigDto
		if err := c.ShouldBindJSON(&input); err != nil {
			presentError(ctx, c, errors.Wrap(models.BadParameterError, err.Error()))
			return
		}

		if err := input.Validate(); err != nil {
			presentError(ctx, c, err)
			return
		}

		createContinuousScreeningConfigInput :=
			dto.AdaptCreateContinuousScreeningConfigDtoToModel(input)
		createContinuousScreeningConfigInput.OrgId = organizationId

		uc := usecasesWithCreds(ctx, uc).NewContinuousScreeningUsecase()
		continuousScreeningConfig, err := uc.CreateContinuousScreeningConfig(ctx, createContinuousScreeningConfigInput)
		if presentError(ctx, c, err) {
			return
		}

		c.JSON(http.StatusCreated, dto.AdaptContinuousScreeningConfigDto(continuousScreeningConfig))
	}
}

func handleUpdateContinuousScreeningConfig(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		stableIdString := c.Param("stable_id")
		stableId, err := uuid.Parse(stableIdString)
		if err != nil {
			presentError(ctx, c, errors.Wrap(models.BadParameterError, err.Error()))
			return
		}

		var input dto.UpdateContinuousScreeningConfigDto
		if err := c.ShouldBindJSON(&input); err != nil {
			presentError(ctx, c, errors.Wrap(models.BadParameterError, err.Error()))
			return
		}
		if err := input.Validate(); err != nil {
			presentError(ctx, c, err)
			return
		}

		uc := usecasesWithCreds(ctx, uc).NewContinuousScreeningUsecase()
		continuousScreeningConfig, err := uc.UpdateContinuousScreeningConfig(
			ctx,
			stableId,
			dto.AdaptUpdateContinuousScreeningConfigDtoToModel(input),
		)
		if presentError(ctx, c, err) {
			return
		}

		c.JSON(http.StatusOK, dto.AdaptContinuousScreeningConfigDto(continuousScreeningConfig))
	}
}

func handleCreateContinuousScreeningObject(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		var input dto.CreateContinuousScreeningObjectDto
		if err := c.ShouldBindJSON(&input); err != nil {
			presentError(ctx, c, errors.Wrap(models.BadParameterError, err.Error()))
			return
		}

		uc := usecasesWithCreds(ctx, uc).NewContinuousScreeningUsecase()
		continuousScreening, err := uc.CreateContinuousScreeningObject(
			ctx,
			dto.AdaptCreateContinuousScreeningObjectDto(input),
		)
		if presentError(ctx, c, err) {
			return
		}

		c.JSON(http.StatusCreated, dto.AdaptContinuousScreeningDto(continuousScreening))
	}
}

var continuousScreeningPaginationDefaults = models.PaginationDefaults{
	Limit:  25,
	SortBy: models.ContinuousScreeningSortingCreatedAt,
	Order:  models.SortingOrderDesc,
}

func handleListContinuousScreeningsForOrg(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		organizationId, err := utils.OrganizationIdFromRequest(c.Request)
		if presentError(ctx, c, err) {
			return
		}

		var paginationAndSortingDto dto.PaginationAndSorting
		if err := c.ShouldBind(&paginationAndSortingDto); err != nil {
			c.JSON(http.StatusBadRequest, dto.APIErrorResponse{
				Message: err.Error(),
			})
			return
		}
		paginationAndSorting := models.WithPaginationDefaults(
			dto.AdaptPaginationAndSorting(paginationAndSortingDto),
			continuousScreeningPaginationDefaults,
		)

		uc := usecasesWithCreds(ctx, uc).NewContinuousScreeningUsecase()
		screenings, err := uc.ListContinuousScreeningsForOrg(ctx, organizationId, paginationAndSorting)
		if presentError(ctx, c, err) {
			return
		}

		c.JSON(
			http.StatusOK,
			pure_utils.Map(
				screenings,
				dto.AdaptContinuousScreeningDto,
			),
		)
	}
}

func handleUpdateContinuousScreeningMatchStatus(uc usecases.Usecases) func(c *gin.Context) {
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

		uc := usecasesWithCreds(ctx, uc).NewContinuousScreeningUsecase()
		match, err := uc.UpdateContinuousScreeningMatchStatus(ctx, update)
		if presentError(ctx, c, err) {
			return
		}

		c.JSON(http.StatusOK, dto.AdaptContinuousScreeningMatchDto(match))
	}
}

func handleDeleteContinuousScreeningObject(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		var input dto.DeleteContinuousScreeningObjectDto
		if err := c.ShouldBindJSON(&input); err != nil {
			presentError(ctx, c, errors.Wrap(models.BadParameterError, err.Error()))
			return
		}

		uc := usecasesWithCreds(ctx, uc).NewContinuousScreeningUsecase()
		err := uc.DeleteContinuousScreeningObject(
			ctx,
			dto.AdaptDeleteContinuousScreeningObjectDto(input),
		)
		if presentError(ctx, c, err) {
			return
		}

		c.Status(http.StatusNoContent)
	}
}

var continuousScreeningObjectsPaginationDefaults = models.PaginationDefaults{
	Limit:  25,
	SortBy: models.SortingFieldCreatedAt,
	Order:  models.SortingOrderDesc,
}

func handleListContinuousScreeningObjects(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		var filtersDto dto.ListContinuousScreeningObjectsFilters
		if err := c.ShouldBind(&filtersDto); err != nil {
			presentError(ctx, c, errors.Wrap(models.BadParameterError, err.Error()))
			return
		}

		var paginationAndSortingDto dto.PaginationAndSorting
		if err := c.ShouldBind(&paginationAndSortingDto); err != nil {
			presentError(ctx, c, errors.Wrap(models.BadParameterError, err.Error()))
			return
		}
		paginationAndSorting := models.WithPaginationDefaults(
			dto.AdaptPaginationAndSorting(paginationAndSortingDto),
			continuousScreeningObjectsPaginationDefaults,
		)

		uc := usecasesWithCreds(ctx, uc).NewContinuousScreeningUsecase()
		objects, err := uc.ListMonitoredObjects(
			ctx,
			dto.AdaptListContinuousScreeningObjectsFiltersDto(filtersDto),
			paginationAndSorting,
		)
		if presentError(ctx, c, err) {
			return
		}

		c.JSON(
			http.StatusOK,
			pure_utils.Map(objects, dto.AdaptContinuousScreeningObjectDto),
		)
	}
}

func handleDismissContinuousScreening(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		continuousScreeningId, err := uuid.Parse(c.Param("id"))
		if err != nil {
			presentError(ctx, c, errors.Wrap(models.BadParameterError, err.Error()))
			return
		}

		creds, ok := utils.CredentialsFromCtx(ctx)
		if !ok {
			presentError(ctx, c, models.ErrUnknownUser)
			return
		}

		uc := usecasesWithCreds(ctx, uc).NewContinuousScreeningUsecase()
		csWithMatches, err := uc.DismissContinuousScreening(ctx, continuousScreeningId, &creds.ActorIdentity.UserId)
		if presentError(ctx, c, err) {
			return
		}

		c.JSON(http.StatusOK, dto.AdaptContinuousScreeningDto(csWithMatches))
	}
}

func handleLoadMoreContinuousScreeningMatches(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		id, err := uuid.Parse(c.Param("id"))
		if err != nil {
			presentError(ctx, c, errors.Wrap(models.BadParameterError, err.Error()))
			return
		}

		uc := usecasesWithCreds(ctx, uc).NewContinuousScreeningUsecase()
		screening, err := uc.LoadMoreContinuousScreeningMatches(ctx, id)
		if presentError(ctx, c, err) {
			return
		}

		c.JSON(http.StatusOK, dto.AdaptContinuousScreeningDto(screening))
	}
}

// Manifest and Dataset
func handleGetContinuousScreeningCatalog(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		usecase := uc.NewContinuousScreeningManifestUsecase()
		catalog, err := usecase.GetContinuousScreeningCatalog(ctx)
		if presentError(ctx, c, err) {
			return
		}

		c.JSON(http.StatusOK, catalog)
	}
}

func handleGetContinuousScreeningDeltaList(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		orgIdStr := c.Param("org_id")
		orgId, err := uuid.Parse(orgIdStr)
		if err != nil {
			presentError(ctx, c, errors.Wrap(models.BadParameterError, err.Error()))
			return
		}

		usecase := uc.NewContinuousScreeningManifestUsecase()
		deltas, err := usecase.GetContinuousScreeningDeltaList(ctx, orgId)

		if presentError(ctx, c, err) {
			return
		}

		c.JSON(http.StatusOK, deltas)
	}
}

func handleGetContinuousScreeningDelta(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		orgIdStr := c.Param("org_id")
		orgId, err := uuid.Parse(orgIdStr)
		if err != nil {
			presentError(ctx, c, errors.Wrap(models.BadParameterError, err.Error()))
			return
		}
		deltaIdStr := c.Param("delta_id")
		deltaId, err := uuid.Parse(deltaIdStr)
		if err != nil {
			presentError(ctx, c, errors.Wrap(models.BadParameterError, err.Error()))
			return
		}
		usecase := uc.NewContinuousScreeningManifestUsecase()
		deltaBlob, err := usecase.GetContinuousScreeningDeltaBlob(ctx, orgId, deltaId)
		if presentError(ctx, c, err) {
			return
		}
		defer deltaBlob.ReadCloser.Close()
		c.DataFromReader(http.StatusOK, -1, "application/x-ndjson", deltaBlob.ReadCloser, nil)
	}
}

func handleGetContinuousScreeningFull(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		orgIdStr := c.Param("org_id")
		orgId, err := uuid.Parse(orgIdStr)
		if err != nil {
			presentError(ctx, c, errors.Wrap(models.BadParameterError, err.Error()))
			return
		}

		usecase := uc.NewContinuousScreeningManifestUsecase()
		fullBlob, err := usecase.GetContinuousScreeningFullBlob(ctx, orgId)
		if presentError(ctx, c, err) {
			return
		}
		defer fullBlob.ReadCloser.Close()
		c.DataFromReader(http.StatusOK, -1, "application/x-ndjson", fullBlob.ReadCloser, nil)
	}
}
