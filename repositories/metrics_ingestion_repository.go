package repositories

import (
	"context"
	"slices"

	"github.com/checkmarble/marble-backend/infra"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories/bqmodels"
	"github.com/cockroachdb/errors"
)

// BigQuery rejects streaming inserts over 10 MB or 50,000 rows per request, and recommends batches of about 500 rows.
const METRICS_INSERT_BATCH_SIZE = 500

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
	return putInBatches(ctx, metricEventRows, METRICS_INSERT_BATCH_SIZE,
		func(ctx context.Context, batch []*bqmodels.MetricEventRow) error {
			return inserter.Put(ctx, batch)
		})
}

// putInBatches inserts the rows batch by batch and stops at the first failing batch. The batches inserted before it
// are kept, so retrying the whole collection duplicates them.
func putInBatches[T any](ctx context.Context, rows []T, batchSize int, put func(context.Context, []T) error) error {
	for batch := range slices.Chunk(rows, batchSize) {
		if err := put(ctx, batch); err != nil {
			return errors.Wrapf(err, "failed to insert a batch of %d metrics out of %d", len(batch), len(rows))
		}
	}
	return nil
}
