package api

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"marble/marble-backend/models"
	"marble/marble-backend/usecases"
	"marble/marble-backend/utils"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"golang.org/x/exp/slog"
)

type API struct {
	app AppInterface

	port     string
	router   *chi.Mux
	usecases usecases.Usecases
	logger   *slog.Logger
}

type AppInterface interface {
	GetDataModel(ctx context.Context, organizationID string) (models.DataModel, error)
}

func New(ctx context.Context, port string, a AppInterface, usecases usecases.Usecases, logger *slog.Logger, corsAllowLocalhost bool) (*http.Server, error) {

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
	r.Use(utils.StoreLoggerInContextMiddleware(logger))
	r.Use(cors.Handler(corsOption(corsAllowLocalhost)))
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			next.ServeHTTP(w, r)
		})
	})

	s := &API{
		app: a,

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
