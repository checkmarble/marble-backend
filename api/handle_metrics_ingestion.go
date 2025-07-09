package api

import (
	"net/http"

	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/usecases"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/gin-gonic/gin"
)

// Handle metrics ingestion from the metrics collection worker.
func handleMetricsIngestion(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		logger := utils.LoggerFromContext(c.Request.Context())
		var metricsCollectionDto dto.MetricsCollectionDto
		if err := c.ShouldBindJSON(&metricsCollectionDto); err != nil {
			c.Status(http.StatusBadRequest)
			logger.WarnContext(c.Request.Context(), "Failed to bind metrics collection", "error", err.Error())
			return
		}

		metricsCollection := dto.AdaptMetricsCollection(metricsCollectionDto)

		usecase := uc.NewMetricsIngestionUsecase()
		err := usecase.IngestMetrics(c.Request.Context(), metricsCollection)
		if presentError(c.Request.Context(), c, err) {
			logger.WarnContext(c.Request.Context(), "Failed to ingest metrics", "error", err.Error())
			return
		}

		// Success response
		c.Status(http.StatusOK)
	}
}
