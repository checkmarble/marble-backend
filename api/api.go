package api

import (
	"fmt"
	"net/http"
	"time"

	"marble/marble-backend/app/data_model"
	"marble/marble-backend/app/dynamic_reading"
	payload_package "marble/marble-backend/app/payload"
	"marble/marble-backend/app/scenarios"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

type API struct {
	app AppInterface

	port   string
	router *chi.Mux
}

type AppInterface interface {
	GetOrganizationIDFromToken(token string) (orgID string, err error)
	GetDataModel(organizationID string) (data_model.DataModel, error)

	PayloadFromTriggerObject(organizationID string, triggerObject map[string]any) (payload_package.Payload, error)
	CreateDecision(organizationID string, scenarioID string, payload payload_package.Payload) (scenarios.Decision, error)
	GetDecision(organizationID string, requestedDecisionID string) (scenarios.Decision, error)
	IngestObject(dynamicStructWithReader dynamic_reading.DynamicStructWithReader, table data_model.Table) (err error)
	ParseToDataModelObject(table data_model.Table, objectBody []byte) (*dynamic_reading.DynamicStructWithReader, error)
}

func New(port string, a AppInterface) (*http.Server, error) {

	///////////////////////////////
	// Setup a router
	///////////////////////////////
	r := chi.NewRouter()

	////////////////////////////////////////////////////////////
	// Middleware
	////////////////////////////////////////////////////////////
	r.Use(middleware.RequestID)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	s := &API{
		app: a,

		port:   port,
		router: r,
	}

	// Setup the routes
	s.routes()

	// display routes for debugging
	s.displayRoutes()

	////////////////////////////////////////////////////////////
	// Setup a go http.Server
	////////////////////////////////////////////////////////////

	// create a go router instance
	srv := &http.Server{
		// adress
		Addr: fmt.Sprintf("0.0.0.0:%s", port),

		// Good practice to set timeouts to avoid Slowloris attacks.
		WriteTimeout: time.Second * 15,
		ReadTimeout:  time.Second * 15,
		IdleTimeout:  time.Second * 60,

		// Instance of gorilla router
		Handler: r,
	}

	return srv, nil
}
