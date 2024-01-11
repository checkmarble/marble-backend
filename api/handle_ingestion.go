package api

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"

	"github.com/cockroachdb/errors"
	"github.com/gin-gonic/gin"

	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/usecases/payload_parser"
	"github.com/checkmarble/marble-backend/utils"
)

func (api *API) handleIngestion(c *gin.Context) {
	logger := utils.LoggerFromContext(c.Request.Context())
	organizationId, err := utils.OrgIDFromCtx(c.Request.Context(), c.Request)
	if presentError(c, err) {
		return
	}

	logger = logger.With(slog.String("organizationId", organizationId))

	usecase := api.UsecasesWithCreds(c.Request).NewIngestionUseCase()

	dataModelUseCase := api.UsecasesWithCreds(c.Request).NewDataModelUseCase()
	dataModel, err := dataModelUseCase.GetDataModel(c.Request.Context(), organizationId)
	if err != nil {
		logger.ErrorContext(c.Request.Context(), fmt.Sprintf("Unable to find datamodel by organizationId for ingestion: %v", err))
		http.Error(c.Writer, "", http.StatusInternalServerError)
		return
	}

	objectType := c.Param("object_type")
	objectBody, err := io.ReadAll(c.Request.Body)
	if err != nil {
		logger.ErrorContext(c.Request.Context(), fmt.Sprintf("Error while reading request body bytes in api handle_ingestion: %v", err))
		http.Error(c.Writer, "", http.StatusUnprocessableEntity)
		return
	}
	logger = logger.With(slog.String("object_type", objectType))

	tables := dataModel.Tables
	table, ok := tables[models.TableName(objectType)]
	if !ok {
		logger.ErrorContext(c.Request.Context(), "Table not found in data model for organization")
		http.Error(c.Writer, "", http.StatusNotFound)
		return
	}

	parser := payload_parser.NewParser()
	validationErrors, err := parser.ValidatePayload(table, objectBody)
	if err != nil {
		presentError(c, errors.Wrap(models.BadParameterError, fmt.Sprintf("Error while validating payload: %v", err)))
		return
	}
	if len(validationErrors) > 0 {
		encoded, _ := json.Marshal(validationErrors)
		logger.InfoContext(c.Request.Context(), fmt.Sprintf("Validation errors on POST ingestion %s: %s", objectType, string(encoded)))
		http.Error(c.Writer, string(encoded), http.StatusBadRequest)
		return
	}

	payload, err := payload_parser.ParseToDataModelObject(table, objectBody)
	if errors.Is(err, models.FormatValidationError) {
		logger.ErrorContext(c.Request.Context(), fmt.Sprintf("format validation error while parsing to data model object: %v", err))
		http.Error(c.Writer, "", http.StatusUnprocessableEntity)
		return
	} else if err != nil {
		logger.ErrorContext(c.Request.Context(), fmt.Sprintf("Unexpected error while parsing to data model object: %v", err))
		http.Error(c.Writer, "", http.StatusInternalServerError)
		return
	}
	err = usecase.IngestObjects(c.Request.Context(), organizationId, []models.PayloadReader{payload}, table, logger)
	if err != nil {
		logger.ErrorContext(c.Request.Context(), fmt.Sprintf("Error while ingesting object: %v", err))
		http.Error(c.Writer, "", http.StatusInternalServerError)
		return
	}
	c.Status(http.StatusCreated)
}

func (api *API) handleCsvIngestion(c *gin.Context) {
	ctx := c.Request.Context()
	creds, found := utils.CredentialsFromCtx(ctx)
	if !found {
		presentError(c, fmt.Errorf("no credentials in context"))
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

	ingestionUseCase := api.UsecasesWithCreds(c.Request).NewIngestionUseCase()
	uploadLog, err := ingestionUseCase.ValidateAndUploadIngestionCsv(ctx, creds.OrganizationId, userId, objectType, fileReader)

	if presentError(c, err) {
		return
	}

	apiUploadLog := dto.AdaptUploadLogDto(uploadLog)
	c.JSON(http.StatusOK, apiUploadLog)
}

func (api *API) handleListUploadLogs(c *gin.Context) {
	ctx := c.Request.Context()
	creds, found := utils.CredentialsFromCtx(ctx)
	if !found {
		presentError(c, fmt.Errorf("no credentials in context"))
		return
	}

	objectType := c.Param("object_type")
	ingestionUseCase := api.UsecasesWithCreds(c.Request).NewIngestionUseCase()
	uploadLogs, err := ingestionUseCase.ListUploadLogs(ctx, creds.OrganizationId, objectType)

	if presentError(c, err) {
		return
	}
	c.JSON(http.StatusOK, utils.Map(uploadLogs, dto.AdaptUploadLogDto))
}
