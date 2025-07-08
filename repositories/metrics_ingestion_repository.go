package repositories

import (
	"context"
	"fmt"

	"cloud.google.com/go/bigquery"
	"github.com/checkmarble/marble-backend/infra"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"
)

type MetricsIngestionRepository struct {
	bqClient *infra.BigQueryClient
}

func NewMetricsIngestionRepository(bqClient *infra.BigQueryClient) MetricsIngestionRepository {
	return MetricsIngestionRepository{
		bqClient: bqClient,
	}
}

func (repo MetricsIngestionRepository) StoreMetrics(ctx context.Context, metrics models.MetricsCollection) error {
	return nil
}

func (repo MetricsIngestionRepository) TestConnection(ctx context.Context) error {
	logger := utils.LoggerFromContext(ctx)

	// Simple query that doesn't require any tables
	q := repo.bqClient.Client.Query("SELECT 1 as test_value")

	it, err := q.Read(ctx)
	if err != nil {
		logger.ErrorContext(ctx, "BigQuery connection test failed", "error", err)
		return fmt.Errorf("failed to connect to BigQuery: %w", err)
	}

	// Read the result to ensure the query actually executed
	var row []bigquery.Value
	err = it.Next(&row)
	if err != nil {
		logger.ErrorContext(ctx, "Failed to read BigQuery test result", "error", err)
		return fmt.Errorf("failed to read from BigQuery: %w", err)
	}

	logger.InfoContext(ctx, "BigQuery connection test successful", "result", row[0])
	return nil
}

func (repo MetricsIngestionRepository) SendMetrics(ctx context.Context, metrics models.MetricsCollection) error {
	logger := utils.LoggerFromContext(ctx)

	logger.DebugContext(ctx, "Sending metrics to BigQuery",
		"collection_id", metrics.CollectionID,
		"metrics_count", len(metrics.Metrics),
	)

	table := repo.bqClient.Client.Dataset("metrics").Table("events")
	inserter := table.Inserter()

	metricEventRows := models.AdaptMetricsCollection(metrics)

	err := inserter.Put(ctx, metricEventRows)
	if err != nil {
		logger.ErrorContext(ctx, "Failed to send metrics to BigQuery", "error", err)
		return fmt.Errorf("failed to send metrics to BigQuery: %w", err)
	}

	return nil
}
