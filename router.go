package main

import (
	"context"
	"net/http"
	"time"

	"github.com/getsentry/sentry-go"
	sentrygin "github.com/getsentry/sentry-go/gin"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"

	"github.com/checkmarble/marble-backend/api"
	"github.com/checkmarble/marble-backend/api/middleware"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"
)

func corsOption(env string) cors.Config {
	allowedOrigins := []string{"*"}

	if env == "development" {
		allowedOrigins = append(allowedOrigins, "http://localhost:3000", "http://localhost:3001", "http://localhost:3002", "http://localhost:5173")
	}

	return cors.Config{
		AllowOrigins:     allowedOrigins,
		AllowMethods:     []string{http.MethodOptions, http.MethodHead, http.MethodGet, http.MethodPost, http.MethodDelete, http.MethodPatch},
		AllowHeaders:     []string{"Authorization", "Content-Type", "X-Api-Key", "baggage", "sentry-trace"},
		AllowCredentials: false,
		MaxAge:           12 * time.Hour,
	}
}

func initRouter(ctx context.Context, conf AppConfiguration, deps dependencies) *gin.Engine {
	if conf.env != "development" {
		gin.SetMode(gin.ReleaseMode)
	}

	logger := utils.LoggerFromContext(ctx)
	loggingMiddleware := middleware.NewLogging(logger, middleware.WithIgnorePath([]string{"/liveness"}))

	r := gin.New()

	if err := sentry.Init(sentry.ClientOptions{
		Dsn:           conf.sentryDsn,
		EnableTracing: true,
		Environment:   conf.env,
		TracesSampler: sentry.TracesSampler(func(ctx sentry.SamplingContext) float64 {
			if ctx.Span.Name == "GET /liveness" {
				return 0.0
			}
			if ctx.Span.Name == "POST /ingestion/:object_type" {
				return 0.1
			}
			if ctx.Span.Name == "POST /decisions" {
				return 0.1
			}
			if ctx.Span.Name == "GET /token" {
				return 0.1
			}
			return 1.0
		}),
		// Experimental - value to be adjusted in prod once volumes go up - relative to the trace sampling rate
		ProfilesSampleRate: 1.0,
	}); err != nil {
		panic(err)
	}

	r.Use(gin.Recovery())
	r.Use(sentrygin.New(sentrygin.Options{Repanic: true}))
	r.Use(cors.New(corsOption(conf.env)))
	if conf.env == "development" {
		// GCP already logs those elements
		r.Use(loggingMiddleware)
	}
	r.Use(otelgin.Middleware("marble-backend"))
	r.Use(utils.StoreLoggerInContextMiddleware(logger))
	r.Use(utils.StoreSegmentClientInContextMiddleware(deps.SegmentClient))
	r.Use(utils.StoreOpenTelemetryTracerInContextMiddleware(deps.OpenTelemetryTracer))

	r.GET("/liveness", api.HandleLivenessProbe)
	r.POST("/crash", api.HandleCrash)
	r.POST("/token", deps.TokenHandler.GenerateToken)

	router := r.Use(deps.Authentication.Middleware)

	router.GET("/data-model", api.HasPermission(models.DATA_MODEL_READ), deps.DataModelHandler.GetDataModel)
	router.POST("/data-model/tables", api.HasPermission(models.DATA_MODEL_WRITE), deps.DataModelHandler.CreateTable)
	router.PATCH("/data-model/tables/:tableID", api.HasPermission(models.DATA_MODEL_WRITE), deps.DataModelHandler.UpdateDataModelTable)
	router.POST("/data-model/links", api.HasPermission(models.DATA_MODEL_WRITE), deps.DataModelHandler.CreateLink)
	router.POST("/data-model/tables/:tableID/fields", api.HasPermission(models.DATA_MODEL_WRITE), deps.DataModelHandler.CreateField)
	router.PATCH("/data-model/fields/:fieldID", api.HasPermission(models.DATA_MODEL_WRITE), deps.DataModelHandler.UpdateDataModelField)
	router.DELETE("/data-model", api.HasPermission(models.DATA_MODEL_WRITE), deps.DataModelHandler.DeleteDataModel)
	router.GET("/data-model/openapi", api.HasPermission(models.DATA_MODEL_READ), deps.DataModelHandler.OpenAPI)

	return r
}
