package main

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"

	"github.com/checkmarble/marble-backend/api"
	"github.com/checkmarble/marble-backend/api/middleware"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"
)

func corsOption(env string) cors.Config {
	allowedOrigins := []string{
		"https://backoffice.staging.checkmarble.com",
		"https://marble-backoffice-production.web.app",
	}

	if env == "DEV" {
		allowedOrigins = append(allowedOrigins, "http://localhost:3000", "http://localhost:3001", "http://localhost:3002", "http://localhost:5173")
	}

	return cors.Config{
		AllowOrigins:     allowedOrigins,
		AllowMethods:     []string{http.MethodOptions, http.MethodHead, http.MethodGet, http.MethodPost, http.MethodDelete, http.MethodPost},
		AllowHeaders:     []string{"Authorization", "Content-Type", "X-Api-Key"},
		AllowCredentials: false,
		MaxAge:           12 * time.Hour,
	}
}

func initRouter(ctx context.Context, conf AppConfiguration, deps dependencies) *gin.Engine {
	if conf.env != "DEV" {
		gin.SetMode(gin.ReleaseMode)
	}

	logger := utils.LoggerFromContext(ctx)
	loggingMiddleware := middleware.NewLogging(logger, middleware.WithIgnorePath([]string{"/liveness"}))

	r := gin.New()

	r.Use(gin.Recovery())
	r.Use(cors.New(corsOption(conf.env)))
	r.Use(loggingMiddleware)
	r.Use(utils.StoreLoggerInContextMiddleware(logger))

	r.GET("/liveness", api.HandleLivenessProbe)
	r.POST("/crash", api.HandleCrash)
	r.POST("/token", deps.TokenHandler.GenerateToken)

	router := r.Use(deps.Authentication.Middleware)

	router.GET("/apikeys", api.HasPermission(models.APIKEY_READ), deps.ApiKeysHandler.GetApiKeys)

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
