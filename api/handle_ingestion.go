package api

import (
	"context"
	"errors"
	"io/ioutil"
	"marble/marble-backend/app"
	"net/http"

	"github.com/go-chi/chi/v5"
	"golang.org/x/exp/slog"
)

type IngestionInterface interface {
	IngestObject(ctx context.Context, dynamicStructWithReader app.DynamicStructWithReader, table app.Table) (err error)
}

func (api *API) handleIngestion() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		orgID, err := orgIDFromCtx(ctx)
		if err != nil {
			http.Error(w, "", http.StatusUnauthorized) // 401
			return
		}
		logger := api.logger.With(slog.String("orgId", orgID))

		dataModel, err := api.app.GetDataModel(ctx, orgID)
		if err != nil {
			logger.ErrorCtx(ctx, "Unable to find datamodel by orgId for ingestion:\n"+err.Error())
			w.WriteHeader(http.StatusInternalServerError) // 500
			return
		}

		object_type := chi.URLParam(r, "object_type")
		object_body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			logger.ErrorCtx(ctx, "Error while reading request body bytes in api handle_ingestion")
			w.WriteHeader(http.StatusUnprocessableEntity)
			return
		}
		logger = logger.With(slog.String("object_type", object_type))

		tables := dataModel.Tables
		table, ok := tables[object_type]
		if !ok {
			logger.ErrorCtx(ctx, "Table not found in data model for organization")
			w.WriteHeader(http.StatusNotFound)
			return
		}

		payloadStructWithReaderPtr, err := app.ParseToDataModelObject(ctx, table, object_body)
		if errors.Is(err, app.ErrFormatValidation) {
			w.WriteHeader(http.StatusUnprocessableEntity)
			return
		} else if err != nil {
			logger.ErrorCtx(ctx, "Unexpected error while parsing to data model object:\n"+err.Error())
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		err = api.app.IngestObject(ctx, *payloadStructWithReaderPtr, table)
		if err != nil {
			logger.ErrorCtx(ctx, "Error while ingesting object:\n"+err.Error())
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusCreated)
	}

}
