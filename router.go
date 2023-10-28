package main

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"

	"github.com/checkmarble/marble-backend/api"
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
		AllowedHeaders:   []string{"Authorization", "Content-Type", "X-Api-Key", "baggage", "sentry-trace"},
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

func initRouter(ctx context.Context, conf AppConfiguration, deps dependencies) *chi.Mux {
	r := chi.NewRouter()

	isDevEnv := conf.env == "DEV"
	if isDevEnv {
		r.Use(middleware.RequestID)
		r.Use(middleware.Logger)
	}
	r.Use(middleware.Recoverer)
	r.Use(utils.StoreLoggerInContextMiddleware(utils.LoggerFromContext(ctx)))
	r.Use(utils.AddTraceIdToLoggerMiddleware(isDevEnv, conf.gcpProject))
	r.Use(cors.Handler(corsOption(isDevEnv)))
	r.Use(setContentTypeMiddleware)

	r.Post("/crash", api.HandleCrash)
	r.Post("/token", deps.TokenHandler.GenerateToken)

	router := r.With(deps.Authentication.Middleware)

	router.Get("/data-model", deps.DataModelHandler.GetDataModel)
	router.Post("/data-model/tables", deps.DataModelHandler.CreateTable)
	router.Patch("/data-model/tables/{tableID}", deps.DataModelHandler.UpdateDataModelTable)
	router.Post("/data-model/links", deps.DataModelHandler.CreateLink)
	router.Post("/data-model/tables/{tableID}/fields", deps.DataModelHandler.CreateField)
	router.Patch("/data-model/fields/{fieldID}", deps.DataModelHandler.UpdateDataModelField)
	router.Delete("/data-model", deps.DataModelHandler.DeleteDataModel)
	router.Get("/data-model/openapi", deps.DataModelHandler.OpenAPI)

	return r
}
