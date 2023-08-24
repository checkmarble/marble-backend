package api

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"marble/marble-backend/models"
	"marble/marble-backend/usecases/payload_parser"
	"marble/marble-backend/utils"
	"net/http"

	"github.com/go-chi/chi/v5"
)

func (api *API) handleIngestion() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		organizationId, err := utils.OrgIDFromCtx(ctx, r)
		if presentError(w, r, err) {
			return
		}

		logger := api.logger.With(slog.String("organizationId", organizationId))

		usecase := api.usecases.NewIngestionUseCase()

		organizationUsecase := api.usecases.NewOrganizationUseCase()
		dataModel, err := organizationUsecase.GetDataModel(organizationId)
		if err != nil {
			logger.ErrorContext(ctx, fmt.Sprintf("Unable to find datamodel by organizationId for ingestion: %v", err))
			http.Error(w, "", http.StatusInternalServerError)
			return
		}

		object_type := chi.URLParam(r, "object_type")
		object_body, err := io.ReadAll(r.Body)
		if err != nil {
			logger.ErrorContext(ctx, fmt.Sprintf("Error while reading request body bytes in api handle_ingestion: %v", err))
			http.Error(w, "", http.StatusUnprocessableEntity)
			return
		}
		logger = logger.With(slog.String("object_type", object_type))

		tables := dataModel.Tables
		table, ok := tables[models.TableName(object_type)]
		if !ok {
			logger.ErrorContext(ctx, "Table not found in data model for organization")
			http.Error(w, "", http.StatusNotFound)
			return
		}

		payload, err := payload_parser.ParseToDataModelObject(table, object_body)
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
