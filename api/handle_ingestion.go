package api

import (
	"errors"
	"io/ioutil"
	"marble/marble-backend/app"
	"marble/marble-backend/models"
	"marble/marble-backend/utils"
	"net/http"

	"github.com/go-chi/chi/v5"
	"golang.org/x/exp/slog"
)

func (api *API) handleIngestion() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		orgID, err := utils.OrgIDFromCtx(ctx, r)
		if presentError(w, r, err) {
			return
		}

		logger := api.logger.With(slog.String("orgId", orgID))

		usecase := api.usecases.NewIngestionUseCase()

		organizationUsecase := api.usecases.NewOrganizationUseCase()
		dataModel, err := organizationUsecase.GetDataModel(orgID)
		if err != nil {
			logger.ErrorCtx(ctx, "Unable to find datamodel by orgId for ingestion:\n"+err.Error())
			http.Error(w, "", http.StatusInternalServerError)
			return
		}

		object_type := chi.URLParam(r, "object_type")
		object_body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			logger.ErrorCtx(ctx, "Error while reading request body bytes in api handle_ingestion")
			http.Error(w, "", http.StatusUnprocessableEntity)
			return
		}
		logger = logger.With(slog.String("object_type", object_type))

		tables := dataModel.Tables
		table, ok := tables[models.TableName(object_type)]
		if !ok {
			logger.ErrorCtx(ctx, "Table not found in data model for organization")
			http.Error(w, "", http.StatusNotFound)
			return
		}

		payload, err := app.ParseToDataModelObject(table, object_body)
		if errors.Is(err, models.FormatValidationError) {
			http.Error(w, "", http.StatusUnprocessableEntity)
			return
		} else if err != nil {
			logger.ErrorCtx(ctx, "Unexpected error while parsing to data model object:\n"+err.Error())
			http.Error(w, "", http.StatusInternalServerError)
			return
		}
		err = usecase.IngestObject(orgID, []models.PayloadReader{payload}, table, logger)
		if err != nil {
			logger.ErrorCtx(ctx, "Error while ingesting object:\n"+err.Error())
			http.Error(w, "", http.StatusInternalServerError)
			return
		}
		PresentNothingStatusCode(w, http.StatusCreated)
	}

}
