package api

import (
	"encoding/csv"
	"fmt"
	"io"
	"net/http"

	"github.com/cockroachdb/errors"
	"github.com/gin-gonic/gin"

	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/usecases"
	"github.com/checkmarble/marble-backend/utils"
)

func handleIngestion(uc usecases.Usecaser) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		organizationId, err := utils.OrganizationIdFromRequest(c.Request)
		if presentError(ctx, c, err) {
			return
		}

		objectType := c.Param("object_type")
		objectBody, err := io.ReadAll(c.Request.Body)
		if err != nil {
			presentError(ctx, c, errors.Wrap(models.BadParameterError, err.Error()))
			return
		}

		usecase := usecasesWithCreds(ctx, uc).NewIngestionUseCase()
		nb, err := usecase.IngestObject(ctx, organizationId, objectType, objectBody)
		if presentError(ctx, c, err) {
			return
		}
		if nb == 0 {
			c.Status(http.StatusOK)
			return
		}
		c.Status(http.StatusCreated)
	}
}

func handleIngestionMultiple(uc usecases.Usecaser) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		organizationId, err := utils.OrganizationIdFromRequest(c.Request)
		if presentError(ctx, c, err) {
			return
		}

		objectType := c.Param("object_type")
		objectBody, err := io.ReadAll(c.Request.Body)
		if err != nil {
			presentError(ctx, c, errors.Wrap(models.BadParameterError, err.Error()))
			return
		}

		usecase := usecasesWithCreds(ctx, uc).NewIngestionUseCase()
		nb, err := usecase.IngestObjects(ctx, organizationId, objectType, objectBody)
		if presentError(ctx, c, err) {
			return
		}
		if nb == 0 {
			c.Status(http.StatusOK)
			return
		}
		c.Status(http.StatusCreated)
	}
}

func handlePostCsvIngestion(uc usecases.Usecaser) func(c *gin.Context) {
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

		file, _, err := c.Request.FormFile("file")
		if err != nil {
			http.Error(c.Writer, err.Error(), http.StatusBadRequest)
			return
		}
		defer file.Close()

		fileReader := csv.NewReader(pure_utils.NewReaderWithoutBom(file))
		objectType := c.Param("object_type")

		ingestionUseCase := usecasesWithCreds(ctx, uc).NewIngestionUseCase()
		uploadLog, err := ingestionUseCase.ValidateAndUploadIngestionCsv(ctx,
			organizationId, userId, objectType, fileReader)

		if presentError(ctx, c, err) {
			return
		}

		apiUploadLog := dto.AdaptUploadLogDto(uploadLog)
		c.JSON(http.StatusOK, apiUploadLog)
	}
}

func handleListUploadLogs(uc usecases.Usecaser) func(c *gin.Context) {
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
