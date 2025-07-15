package repositories

import (
	"context"
	"fmt"

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

func (repo MetricsIngestionRepository) SendMetrics(ctx context.Context, metrics models.MetricsCollection) error {
	logger := utils.LoggerFromContext(ctx)

	logger.DebugContext(ctx, "Sending metrics to BigQuery",
		"collection_id", metrics.CollectionID,
		"metrics_count", len(metrics.Metrics),
	)

	table := repo.bqClient.Client.Dataset(repo.bqClient.Config.MetricsDataset).Table(repo.bqClient.Config.MetricsTable)
	inserter := table.Inserter()

	metricEventRows := models.AdaptMetricsCollection(metrics)

	err := inserter.Put(ctx, metricEventRows)
	if err != nil {
		logger.ErrorContext(ctx, "Failed to send metrics to BigQuery", "error", err)
		return fmt.Errorf("failed to send metrics to BigQuery: %w", err)
	}

	return nil
}
