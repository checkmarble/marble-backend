package usecases

import (
	"context"
	"fmt"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"
)

type MetricsIngestionRepository interface {
	SendMetrics(ctx context.Context, collection models.MetricsCollection) error
}

type MetricsIngestionUsecase struct {
	repository MetricsIngestionRepository
}

func NewMetricsIngestionUsecase(repository MetricsIngestionRepository) MetricsIngestionUsecase {
	return MetricsIngestionUsecase{
		repository: repository,
	}
}

func (u *MetricsIngestionUsecase) IngestMetrics(ctx context.Context, collection models.MetricsCollection) error {
	logger := utils.LoggerFromContext(ctx)
	logger.DebugContext(ctx, "Sending metrics to BigQuery", "collection", collection)

	err := u.repository.SendMetrics(ctx, collection)
	if err != nil {
		logger.ErrorContext(ctx, "Failed to send metrics to BigQuery", "error", err.Error())
		return fmt.Errorf("failed to send metrics to BigQuery: %s", err.Error())
	}

	return nil
}
