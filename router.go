package main

import (
	"context"
	"net/http"
	"time"

	sentrygin "github.com/getsentry/sentry-go/gin"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"

	"github.com/checkmarble/marble-backend/api/middleware"
	"github.com/checkmarble/marble-backend/utils"
)

func corsOption(conf AppConfiguration) cors.Config {
	protocol := "https://"
	if conf.env == "development" {
		protocol = "http://"
	}

	allowedOrigins := []string{
		protocol + conf.config.MarbleAppHost,
		protocol + conf.config.MarbleBackofficeHost,
	}

	if conf.env == "development" {
		allowedOrigins = append(allowedOrigins,
			"http://localhost:3000", "http://localhost:3001", "http://localhost:3002",
			"http://localhost:3003", "http://localhost:5173")
	}

	return cors.Config{
		AllowOrigins: allowedOrigins,
		AllowMethods: []string{
			http.MethodOptions, http.MethodHead, http.MethodGet,
			http.MethodPost, http.MethodDelete, http.MethodPatch,
		},
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

	r := gin.New()

	r.Use(gin.Recovery())
	r.Use(sentrygin.New(sentrygin.Options{Repanic: true}))
	r.Use(cors.New(corsOption(conf)))
	r.Use(middleware.NewLogging(logger, conf.requestLoggingLevel))
	r.Use(utils.StoreLoggerInContextMiddleware(logger))
	r.Use(utils.StoreSegmentClientInContextMiddleware(deps.SegmentClient))
	r.Use(otelgin.Middleware(
		conf.appName,
		otelgin.WithTracerProvider(deps.TelemetryRessources.TracerProvider),
		otelgin.WithPropagators(deps.TelemetryRessources.TextMapPropagator),
	))
	r.Use(utils.StoreOpenTelemetryTracerInContextMiddleware(deps.TelemetryRessources.Tracer))

	return r
}
