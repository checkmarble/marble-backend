package scheduled_execution

import (
	"context"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/riverqueue/river"
)

const (
	CSV_INGESTION_TIMEOUT = 1 * time.Hour
)

type CsvIngestionUsecase interface {
	IngestDataFromCsvByUploadLogId(ctx context.Context, uploadLogId string) error
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
	return w.ingestionUsecase.IngestDataFromCsvByUploadLogId(ctx, job.Args.UploadLogId)
}
