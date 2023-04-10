package api

import (
	"errors"
	"io/ioutil"
	"log"
	"marble/marble-backend/app"
	"net/http"

	"github.com/go-chi/chi/v5"
)

func (a *API) handleIngestion() http.HandlerFunc {

	///////////////////////////////
	// Request and Response types defined in scope
	///////////////////////////////

	// return is a decision

	return func(w http.ResponseWriter, r *http.Request) {

		///////////////////////////////
		// Authorize request
		///////////////////////////////
		orgID, err := orgIDFromCtx(r.Context())
		if err != nil {
			http.Error(w, "", http.StatusUnauthorized) // 401
			return
		}

		dataModel, err := a.app.GetDataModel(orgID)
		if err != nil {
			log.Printf("Unable to find datamodel by orgId for ingestion: %v", err)
			http.Error(w, "No data model found for this organization ID.", http.StatusInternalServerError) // 500
			return
		}

		object_type := chi.URLParam(r, "object_type")
		object_body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			log.Printf("Error while reading request body bytes in api handle_ingestion: %s", err)
			http.Error(w, "", http.StatusUnprocessableEntity) // 422
			return
		}
		// TODO: remove this before production
		log.Printf("Received object type: %s\n", object_type)
		log.Printf("Received object body: %s\n", object_body)

		tables := dataModel.Tables
		table, ok := tables[object_type]
		if !ok {
			log.Printf("Table %s not found in data model for organization %s", object_type, orgID)
			http.Error(w, "No data model found for this object type.", http.StatusNotFound) // 404
			return
		}

		payloadStructWithReaderPtr, err := app.ParseToDataModelObject(table, object_body)
		if err != nil {
			if errors.Is(err, app.ErrFormatValidation) {
				http.Error(w, "Format validation error", http.StatusUnprocessableEntity) // 422
				return
			}
			log.Printf("Unexpected error while parsing to data model object: %v", err)
			http.Error(w, "", http.StatusInternalServerError) // 500
			return
		}

		err = a.app.IngestObject(*payloadStructWithReaderPtr, table)
		if err != nil {
			log.Printf("Error while ingesting object: %v", err)
			http.Error(w, "", http.StatusInternalServerError) // 500
			return
		}
		w.WriteHeader(http.StatusCreated) // 201
		return
	}

}
