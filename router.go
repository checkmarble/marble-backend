package main

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"

	"github.com/checkmarble/marble-backend/utils"
)

func corsOption(corsAllowLocalhost bool) cors.Options {
	allowedOrigins := []string{"https://*"}
	if corsAllowLocalhost {
		allowedOrigins = append(allowedOrigins, "http://localhost:3000", "http://localhost:3001", "http://localhost:3002", "http://localhost:5173")
	}
	return cors.Options{
		AllowedOrigins:   allowedOrigins,
		AllowedMethods:   []string{"GET", "PATCH", "POST", "PUT", "DELETE"},
		AllowedHeaders:   []string{"Authorization", "Content-Type", "X-Api-Key"},
		AllowCredentials: false,
		MaxAge:           7200, // Maximum value not ignored by any of major browsers
	}
}

func setContentTypeMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		next.ServeHTTP(w, r)
	})
}

func initRouter(ctx context.Context, isDevEnv bool, projectID string) *chi.Mux {
	r := chi.NewRouter()

	if isDevEnv {
		r.Use(middleware.RequestID)
		r.Use(middleware.Logger)
	}
	r.Use(middleware.Recoverer)
	r.Use(utils.StoreLoggerInContextMiddleware(utils.LoggerFromContext(ctx)))
	r.Use(utils.AddTraceIdToLoggerMiddleware(isDevEnv, projectID))
	r.Use(cors.Handler(corsOption(isDevEnv)))
	r.Use(setContentTypeMiddleware)
	return r
}
