package api

import (
	"context"
	"errors"
	"io/ioutil"
	"marble/marble-backend/app"
	"marble/marble-backend/utils"
	"net/http"

	"github.com/go-chi/chi/v5"
	"golang.org/x/exp/slog"
)

type IngestionInterface interface {
	IngestObject(ctx context.Context, dynamicStructWithReader app.DynamicStructWithReader, table app.Table, logger *slog.Logger) (err error)
}

func (api *API) handleIngestion() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		orgID, err := utils.OrgIDFromCtx(ctx)
		if err != nil {
			http.Error(w, "", http.StatusUnauthorized) // 401
			return
		}
		logger := api.logger.With(slog.String("orgId", orgID))

		dataModel, err := api.app.GetDataModel(ctx, orgID)
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
		table, ok := tables[app.TableName(object_type)]
		if !ok {
			logger.ErrorCtx(ctx, "Table not found in data model for organization")
			http.Error(w, "", http.StatusNotFound)
			return
		}

		payloadStructWithReader, err := app.ParseToDataModelObject(ctx, table, object_body)
		if errors.Is(err, app.ErrFormatValidation) {
			http.Error(w, "", http.StatusUnprocessableEntity)
			return
		} else if err != nil {
			logger.ErrorCtx(ctx, "Unexpected error while parsing to data model object:\n"+err.Error())
			http.Error(w, "", http.StatusInternalServerError)
			return
		}

		err = api.app.IngestObject(ctx, payloadStructWithReader, table, logger)
		if err != nil {
			logger.ErrorCtx(ctx, "Error while ingesting object:\n"+err.Error())
			http.Error(w, "", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusCreated)
	}

}
