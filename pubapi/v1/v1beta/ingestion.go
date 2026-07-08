package v1beta

import (
	"net/http"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pubapi"
	"github.com/checkmarble/marble-backend/pubapi/types"
	"github.com/checkmarble/marble-backend/pubapi/v1/dto"
	"github.com/checkmarble/marble-backend/pubapi/v1/params"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/usecases"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func HandleUploadCsv(uc usecases.Usecases) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		orgId, err := utils.OrganizationIdFromRequest(c.Request)
		if err != nil {
			types.NewErrorResponse().WithError(err).Serve(c)
			return
		}

		recordType := c.Param("objectType")

		var p params.IngestionParams

		if err := c.ShouldBindQuery(&p); err != nil {
			types.NewErrorResponse().WithError(err).Serve(c)
			return
		}

		ingestionOptions := models.IngestionOptions{
			ShouldMonitor:          p.MonitorObjects,
			ShouldScreen:           p.MonitorObjects && !p.SkipInitialScreening,
			ContinuousScreeningIds: make([]uuid.UUID, len(p.ContinuousConfigIds)),
		}

		if p.MonitorObjects {
			for idx, configId := range p.ContinuousConfigIds {
				ingestionOptions.ContinuousScreeningIds[idx] = uuid.MustParse(configId)
			}
		}

		uc := pubapi.UsecasesWithCreds(ctx, uc).NewIngestionUseCase()
		signedUrl, err := uc.GenerateUploadLink(ctx, orgId, recordType, ingestionOptions, c.Request.Header)
		if err != nil {
			types.NewErrorResponse().WithError(err).Serve(c)
			return
		}

		types.
			NewResponse(dto.AsyncUpload{
				UploadUrl: signedUrl,
			}).
			Serve(c, http.StatusAccepted)
	}
}

var uploadLogPaginationDefaults = models.PaginationDefaults{
	Limit:  25,
	SortBy: models.DecisionSortingCreatedAt,
	Order:  models.SortingOrderDesc,
}

func HandleBatchIngestionLog(uc usecases.Usecases) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		orgId, err := utils.OrganizationIdFromRequest(c.Request)
		if err != nil {
			types.NewErrorResponse().WithError(err).Serve(c)
			return
		}

		recordType := c.Param("objectType")

		var p params.UploadLogParams

		if err := c.ShouldBindQuery(&p); err != nil {
			types.NewErrorResponse().WithError(err).Serve(c)
			return
		}

		ingestionUsecase := pubapi.UsecasesWithCreds(ctx, uc).NewIngestionUseCase()

		paging := p.PaginationParams.ToModel(uploadLogPaginationDefaults)
		paging.Order = models.SortingOrderDesc

		logs, err := ingestionUsecase.ListFilteredUploadLogs(ctx, orgId, recordType, p.ToModel(), paging)
		if err != nil {
			types.NewErrorResponse().WithError(err).Serve(c)
		}

		nextPageId := ""

		if len(logs.Items) > 0 {
			nextPageId = logs.Items[len(logs.Items)-1].Id.String()
		}

		types.NewResponse(pure_utils.Map(logs.Items, dto.AdaptUploadLog)).
			WithPagination(logs.HasNextPage, nextPageId).
			Serve(c)
	}
}
