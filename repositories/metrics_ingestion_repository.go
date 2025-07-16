package repositories

import (
	"context"

	"github.com/checkmarble/marble-backend/infra"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories/bqmodels"
	"github.com/cockroachdb/errors"
)

type MetricsIngestionRepository struct {
	bqInfra *infra.BigQueryInfra
}

func NewMetricsIngestionRepository(bqInfra *infra.BigQueryInfra) MetricsIngestionRepository {
	return MetricsIngestionRepository{
		bqInfra: bqInfra,
	}
}

func (repo MetricsIngestionRepository) SendMetrics(ctx context.Context, metrics models.MetricsCollection) error {
	if repo.bqInfra == nil || repo.bqInfra.MetricsTable == nil {
		return errors.New("bigquery infra is not initialized")
	}

	inserter := repo.bqInfra.MetricsTable.Inserter()
	metricEventRows := bqmodels.AdaptMetricsCollection(metrics)
	err := inserter.Put(ctx, metricEventRows)
	if err != nil {
		return err
	}

	return nil
}
