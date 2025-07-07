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
			return
		}

		// Debug log the body
		logger.DebugContext(c.Request.Context(), "Metrics collection received",
			"metrics_collection", metricsCollectionDto,
		)

		metricsCollection := dto.AdaptMetricsCollection(metricsCollectionDto)

		usecase := uc.NewMetricsIngestionUsecase()
		err := usecase.IngestMetrics(c.Request.Context(), metricsCollection)
		if presentError(c.Request.Context(), c, err) {
			logger.WarnContext(c.Request.Context(), "Failed to ingest metrics", "error", err)
			return
		}

		// Success response
		c.JSON(http.StatusOK, gin.H{
			"status":        "success",
			"message":       "Metrics collection processed successfully",
			"collection_id": metricsCollection.CollectionID,
			"metrics_count": len(metricsCollection.Metrics),
		})
	}
}
