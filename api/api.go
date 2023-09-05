package api

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/usecases"
	"github.com/checkmarble/marble-backend/utils"
)

type API struct {
	router   *chi.Mux
	usecases usecases.Usecases
}

func New(ctx context.Context, port string, usecases usecases.Usecases, isDevEnv bool, projectId string) (*http.Server, error) {

	///////////////////////////////
	// Setup a router
	///////////////////////////////
	r := chi.NewRouter()

	logger := utils.LoggerFromContext(ctx)

	////////////////////////////////////////////////////////////
	// Middleware
	////////////////////////////////////////////////////////////
	if isDevEnv {
		// GCP already does that when server is running on Cloud Run
		r.Use(middleware.RequestID)
		r.Use(middleware.Logger)
	}

	r.Use(middleware.Recoverer)
	r.Use(utils.StoreLoggerInContextMiddleware(logger))
	r.Use(utils.AddTraceIdToLoggerMiddleware(isDevEnv, projectId))
	r.Use(cors.Handler(corsOption(isDevEnv)))
	r.Use(setContentTypeMiddleware)

	s := &API{
		router:   r,
		usecases: usecases,
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

		// Instance of chi router
		Handler: r,
	}

	return srv, nil
}

func (api *API) UsecasesWithCreds(r *http.Request) *usecases.UsecasesWithCreds {
	ctx := r.Context()

	creds := utils.CredentialsFromCtx(ctx)

	// marble admin can specify on which organization to operate
	// Ignore error, empty organizationId is fine, this is not the place to enforce security
	organizationId, _ := utils.OrganizationIdFromRequest(r)

	return &usecases.UsecasesWithCreds{
		Usecases:    api.usecases,
		Credentials: creds,
		Logger:      utils.LoggerFromContext(ctx),
		OrganizationIdOfContext: func() (string, error) {
			if organizationId == "" {
				return "", fmt.Errorf(
					"no OrganizationId for %s in this context. MarbleAdmin can specify one using 'organization-id' query param. %w",
					creds.ActorIdentityDescription(),
					models.BadParameterError,
				)
			}
			return organizationId, nil
		},
		Context: ctx,
	}
}

func setContentTypeMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		next.ServeHTTP(w, r)
	})
}
