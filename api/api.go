package api

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"marble/marble-backend/usecases"
	"marble/marble-backend/utils"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"golang.org/x/exp/slog"
)

type API struct {
	port     string
	router   *chi.Mux
	usecases usecases.Usecases
	logger   *slog.Logger
}

func New(ctx context.Context, port string, usecases usecases.Usecases, corsAllowLocalhost bool) (*http.Server, error) {

	///////////////////////////////
	// Setup a router
	///////////////////////////////
	r := chi.NewRouter()

	logger := utils.LoggerFromContext(ctx)

	////////////////////////////////////////////////////////////
	// Middleware
	////////////////////////////////////////////////////////////
	r.Use(middleware.RequestID)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(utils.StoreLoggerInContextMiddleware(logger))
	r.Use(cors.Handler(corsOption(corsAllowLocalhost)))
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			next.ServeHTTP(w, r)
		})
	})

	s := &API{
		port:     port,
		router:   r,
		usecases: usecases,
		logger:   logger,
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

func (api *API) UsecasesWithCreds(r *http.Request) *usecases.UsecasesWithCreds {
	ctx := r.Context()

	creds := utils.CredentialsFromCtx(ctx)

	// marble admin can specify on which organization to operate
	// Ignore error, empty organizationId is fine, this is not the place to enforce security
	organizationId, _ := utils.OrganizationIdFromRequest(r)

	return &usecases.UsecasesWithCreds{
		Usecases:                api.usecases,
		Credentials:             creds,
		Logger:                  utils.LoggerFromContext(ctx),
		OrganizationIdOfContext: organizationId,
	}
}
