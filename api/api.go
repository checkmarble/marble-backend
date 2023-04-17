package api

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"marble/marble-backend/app"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

type API struct {
	app AppInterface

	port   string
	router *chi.Mux
}

type AppInterface interface {
	ScenarioAppInterface
	ScenarioIterationAppInterface
	OrganizationAppInterface

	GetOrganizationIDFromToken(ctx context.Context, token string) (orgID string, err error)
	GetDataModel(ctx context.Context, organizationID string) (app.DataModel, error)

	CreateDecision(ctx context.Context, organizationID string, scenarioID string, dynamicStructWithReader app.DynamicStructWithReader, payload app.Payload) (app.Decision, error)
	GetDecision(ctx context.Context, organizationID string, requestedDecisionID string) (app.Decision, error)
	IngestObject(ctx context.Context, dynamicStructWithReader app.DynamicStructWithReader, table app.Table) (err error)
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
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			next.ServeHTTP(w, r)
		})
	})

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
