package api

import (
	"fmt"
	"io/ioutil"
	"log"
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
			http.Error(w, "", http.StatusUnauthorized)
			return
		}

		object_type := chi.URLParam(r, "object_type")
		fmt.Printf("Received object type: %s\n", object_type)

		object_body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			panic(err)
		}

		fmt.Printf("Received object body: %s\n", object_body)

		dataModel, err := a.app.GetDataModel(orgID)
		if err != nil {
			log.Fatalf("Unable to find datamodel by orgId for ingestion: %v", err)
		}

		tables := dataModel.Tables
		table, ok := tables[object_type]
		if !ok {
			log.Fatalf("table %s not found in data model", object_type)
		}

		payloadStructWithReaderPtr, err := a.app.ParseToDataModelObject(table, object_body)
		if err != nil {
			log.Fatalf("Error while parsing struct in api handle_ingestion: %v", err)
		}

		a.app.IngestObject(*payloadStructWithReaderPtr, table)
	}

}
