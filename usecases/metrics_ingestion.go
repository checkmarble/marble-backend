package usecases

import (
	"context"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"
)

type MetricsIngestionUsecase struct{}

func NewMetricsIngestionUsecase() MetricsIngestionUsecase {
	return MetricsIngestionUsecase{}
}

func (u *MetricsIngestionUsecase) IngestMetrics(ctx context.Context, metrics models.MetricsCollection) error {
	// Ingest the collection
	err := u.ingestCollection(ctx, metrics)
	if err != nil {
		return err
	}

	return nil
}

func (u *MetricsIngestionUsecase) ingestCollection(ctx context.Context, collection models.MetricsCollection) error {
	logger := utils.LoggerFromContext(ctx)
	logger.DebugContext(ctx, "Ingesting collection", "collection", collection)

	// TODO: Implement the ingestion logic

	return nil
}
