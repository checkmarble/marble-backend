package usecases

import (
	"context"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/google/uuid"
)

type MetricsIngestionUsecase struct{}

func NewMetricsIngestionUsecase() MetricsIngestionUsecase {
	return MetricsIngestionUsecase{}
}

func (u *MetricsIngestionUsecase) IngestMetrics(ctx context.Context, metrics models.MetricsCollection) error {
	logger := utils.LoggerFromContext(ctx)

	// Check if the collection is already ingested
	alreadyIngested, err := u.isCollectionAlreadyIngested(ctx, metrics.CollectionID)
	if err != nil {
		return err
	}
	if alreadyIngested {
		logger.InfoContext(ctx, "Collection already ingested", "collection_id", metrics.CollectionID)
		return nil
	}

	// Ingest the collection
	err = u.ingestCollection(ctx, metrics)
	if err != nil {
		return err
	}

	return nil
}

func (u *MetricsIngestionUsecase) isCollectionAlreadyIngested(_ context.Context, _ uuid.UUID) (bool, error) {
	// TODO: Implement the logic to check if the collection is already ingested
	return false, nil
}

func (u *MetricsIngestionUsecase) ingestCollection(ctx context.Context, collection models.MetricsCollection) error {
	logger := utils.LoggerFromContext(ctx)
	logger.DebugContext(ctx, "Ingesting collection", "collection", collection)

	// TODO: Implement the ingestion logic

	return nil
}
