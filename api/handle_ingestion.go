package api

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"

	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/usecases/payload_parser"
	"github.com/checkmarble/marble-backend/utils"

	"github.com/go-chi/chi/v5"
)

func (api *API) handleIngestion() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		logger := utils.LoggerFromContext(ctx)
		organizationId, err := utils.OrgIDFromCtx(ctx, r)
		if presentError(w, r, err) {
			return
		}

		logger = logger.With(slog.String("organizationId", organizationId))

		usecase := api.UsecasesWithCreds(r).NewIngestionUseCase()

		dataModelUseCase := api.UsecasesWithCreds(r).NewDataModelUseCase()
		dataModel, err := dataModelUseCase.GetDataModel(organizationId)
		if err != nil {
			logger.ErrorContext(ctx, fmt.Sprintf("Unable to find datamodel by organizationId for ingestion: %v", err))
			http.Error(w, "", http.StatusInternalServerError)
			return
		}

		objectType := chi.URLParam(r, "object_type")
		objectBody, err := io.ReadAll(r.Body)
		if err != nil {
			logger.ErrorContext(ctx, fmt.Sprintf("Error while reading request body bytes in api handle_ingestion: %v", err))
			http.Error(w, "", http.StatusUnprocessableEntity)
			return
		}
		logger = logger.With(slog.String("object_type", objectType))

		tables := dataModel.Tables
		table, ok := tables[models.TableName(objectType)]
		if !ok {
			logger.ErrorContext(ctx, "Table not found in data model for organization")
			http.Error(w, "", http.StatusNotFound)
			return
		}

		parser := payload_parser.NewParser()
		validationErrors, err := parser.ValidatePayload(table, objectBody)
		if err != nil {
			http.Error(w, "", http.StatusUnprocessableEntity)
			return
		}
		if len(validationErrors) > 0 {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(validationErrors)
			return
		}

		payload, err := payload_parser.ParseToDataModelObject(table, objectBody)
		if errors.Is(err, models.FormatValidationError) {
			logger.ErrorContext(ctx, fmt.Sprintf("format validation error while parsing to data model object: %v", err))
			http.Error(w, "", http.StatusUnprocessableEntity)
			return
		} else if err != nil {
			logger.ErrorContext(ctx, fmt.Sprintf("Unexpected error while parsing to data model object: %v", err))
			http.Error(w, "", http.StatusInternalServerError)
			return
		}
		err = usecase.IngestObjects(organizationId, []models.PayloadReader{payload}, table, logger)
		if err != nil {
			logger.ErrorContext(ctx, fmt.Sprintf("Error while ingesting object: %v", err))
			http.Error(w, "", http.StatusInternalServerError)
			return
		}
		PresentNothingStatusCode(w, http.StatusCreated)
	}

}

func (api *API) handleCsvIngestion() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		creds := utils.CredentialsFromCtx(ctx)
		userId := string(creds.ActorIdentity.UserId)

		// Optional: check max size

		file, _, err := r.FormFile("file")
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		defer file.Close()

		fileReader := csv.NewReader(pure_utils.NewReaderWithoutBom(file))
		objectType := chi.URLParam(r, "objectType")

		ingestionUseCase := api.UsecasesWithCreds(r).NewIngestionUseCase()
		uploadLog, err := ingestionUseCase.ValidateAndUploadIngestionCsv(ctx, creds.OrganizationId, userId, objectType, fileReader)

		if presentError(w, r, err) {
			return
		}

		apiUploadLog := dto.AdaptUploadLogDto(uploadLog)
		PresentModel(w, apiUploadLog)
	}
}

func (api *API) handleListUploadLogs() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		creds := utils.CredentialsFromCtx(ctx)

		objectType := chi.URLParam(r, "objectType")
		ingestionUseCase := api.UsecasesWithCreds(r).NewIngestionUseCase()
		uploadLogs, err := ingestionUseCase.ListUploadLogs(ctx, creds.OrganizationId, objectType)

		if presentError(w, r, err) {
			return
		}
		PresentModel(w, utils.Map(uploadLogs, dto.AdaptUploadLogDto))
	}
}
