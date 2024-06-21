package api

import (
	"context"
	"net/http"
	"time"

	sentrygin "github.com/getsentry/sentry-go/gin"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/segmentio/analytics-go/v3"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"

	"github.com/checkmarble/marble-backend/api/middleware"
	"github.com/checkmarble/marble-backend/infra"
	"github.com/checkmarble/marble-backend/utils"
)

func corsOption(conf Configuration) cors.Config {
	protocol := "https://"
	if conf.Env == "development" {
		protocol = "http://"
	}

	allowedOrigins := []string{
		protocol + conf.MarbleAppHost,
		protocol + conf.MarbleBackofficeHost,
	}

	if conf.Env == "development" {
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

func InitRouter(
	ctx context.Context,
	conf Configuration,
	segmentClient analytics.Client,
	telemetryRessources infra.TelemetryRessources,
) *gin.Engine {
	if conf.Env != "development" {
		gin.SetMode(gin.ReleaseMode)
	}

	logger := utils.LoggerFromContext(ctx)

	r := gin.New()

	r.Use(gin.Recovery())
	r.Use(sentrygin.New(sentrygin.Options{Repanic: true}))
	r.Use(cors.New(corsOption(conf)))
	r.Use(middleware.NewLogging(logger, conf.RequestLoggingLevel))
	r.Use(utils.StoreLoggerInContextMiddleware(logger))
	r.Use(utils.StoreSegmentClientInContextMiddleware(segmentClient))
	r.Use(otelgin.Middleware(
		conf.AppName,
		otelgin.WithTracerProvider(telemetryRessources.TracerProvider),
		otelgin.WithPropagators(telemetryRessources.TextMapPropagator),
	))
	r.Use(utils.StoreOpenTelemetryTracerInContextMiddleware(telemetryRessources.Tracer))

	return r
}
