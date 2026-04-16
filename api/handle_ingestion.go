package api

import (
	"encoding/csv"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pubapi/v1/params"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/usecases"
	"github.com/checkmarble/marble-backend/utils"
)

func handlePostCsvIngestion(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		organizationId, err := utils.OrganizationIdFromRequest(c.Request)
		if presentError(ctx, c, err) {
			return
		}
		creds, found := utils.CredentialsFromCtx(ctx)
		if !found {
			presentError(ctx, c, fmt.Errorf("no credentials in context"))
			return
		}
		userId := string(creds.ActorIdentity.UserId)

		var p params.IngestionParams

		if err := c.ShouldBindQuery(&p); presentError(ctx, c, err) {
			return
		}

		file, _, err := c.Request.FormFile("file")
		if err != nil {
			http.Error(c.Writer, err.Error(), http.StatusBadRequest)
			return
		}
		defer file.Close()

		fileReader := csv.NewReader(pure_utils.NewReaderWithoutBom(file))
		objectType := c.Param("object_type")

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

		ingestionUseCase := usecasesWithCreds(ctx, uc).NewIngestionUseCase()
		uploadLog, err := ingestionUseCase.ValidateAndUploadIngestionCsv(ctx,
			organizationId, userId, objectType, fileReader, ingestionOptions)

		if presentError(ctx, c, err) {
			return
		}

		apiUploadLog := dto.AdaptUploadLogDto(uploadLog)
		c.JSON(http.StatusOK, apiUploadLog)
	}
}

func handleListUploadLogs(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		creds, found := utils.CredentialsFromCtx(ctx)
		if !found {
			presentError(ctx, c, fmt.Errorf("no credentials in context"))
			return
		}

		objectType := c.Param("object_type")
		ingestionUseCase := usecasesWithCreds(ctx, uc).NewIngestionUseCase()
		uploadLogs, err := ingestionUseCase.ListUploadLogs(ctx, creds.OrganizationId, objectType)

		if presentError(ctx, c, err) {
			return
		}
		c.JSON(http.StatusOK, pure_utils.Map(uploadLogs, dto.AdaptUploadLogDto))
	}
}
