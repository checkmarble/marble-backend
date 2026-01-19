package scheduled_execution

import (
	"context"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/google/uuid"
	"github.com/riverqueue/river"
)

const (
	CSV_INGESTION_INTERVAL = 1 * time.Minute
	CSV_INGESTION_TIMEOUT  = 1 * time.Hour
)

func NewCsvIngestionPeriodicJob(orgId uuid.UUID) *river.PeriodicJob {
	return river.NewPeriodicJob(
		river.PeriodicInterval(CSV_INGESTION_INTERVAL),
		func() (river.JobArgs, *river.InsertOpts) {
			return models.CsvIngestionArgs{OrgId: orgId},
				&river.InsertOpts{
					Queue: orgId.String(),
					UniqueOpts: river.UniqueOpts{
						ByQueue:  true,
						ByPeriod: CSV_INGESTION_INTERVAL,
					},
				}
		},
		&river.PeriodicJobOpts{RunOnStart: true},
	)
}

type CsvIngestionUsecase interface {
	IngestDataFromCsvForOrg(ctx context.Context, orgId uuid.UUID) error
}

type CsvIngestionWorker struct {
	river.WorkerDefaults[models.CsvIngestionArgs]

	ingestionUsecase CsvIngestionUsecase
}

func NewCsvIngestionWorker(
	ingestionUsecase CsvIngestionUsecase,
) *CsvIngestionWorker {
	return &CsvIngestionWorker{
		ingestionUsecase: ingestionUsecase,
	}
}

func (w *CsvIngestionWorker) Timeout(job *river.Job[models.CsvIngestionArgs]) time.Duration {
	return CSV_INGESTION_TIMEOUT
}

func (w *CsvIngestionWorker) Work(ctx context.Context, job *river.Job[models.CsvIngestionArgs]) error {
	return w.ingestionUsecase.IngestDataFromCsvForOrg(ctx, job.Args.OrgId)
}
