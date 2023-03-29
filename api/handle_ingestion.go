package api

import (
	"fmt"
	"io/ioutil"
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
			http.Error(w, "", http.StatusUnauthorized)
			return
		}

		object_type := chi.URLParam(r, "object_type")
		fmt.Printf("Received object type: %s\n", object_type)

		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			panic(err)
		}

		object_body := body
		fmt.Printf("Received object body: %s\n", object_body)

		a.app.IngestObject(orgID, app.IngestPayload{ObjectType: object_type, ObjectBody: object_body})

	}

}
