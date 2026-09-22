package worker_jobs

import (
	"context"
	"fmt"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
	"github.com/riverqueue/river"
	"gocloud.dev/gcerrors"
)

const (
	ASYNC_UPLOAD_START_TIMEOUT = time.Minute
	ASYNC_UPLOAD_TIMEOUT       = 15 * time.Minute
	ASYNC_UPLOAD_TICK          = time.Minute
	ASYNC_UPLOAD_MAX_SIZE      = 10 << 30 // 10 GB
)

type asyncUploadTaskEnqueuer interface {
	EnqueueCsvIngestionTask(
		ctx context.Context,
		tx repositories.Transaction,
		organizationId uuid.UUID,
		uploadLogId uuid.UUID,
		ingestionOptions models.IngestionOptions,
	) error
}

type asyncUploadFinalizer interface {
	FailUploadLog(ctx context.Context, uploadLogId uuid.UUID, failureCode models.IngestionFailureCode, reason string) error
}

type AsyncUploadWorker struct {
	river.WorkerDefaults[models.AsyncUploadArgs]

	transactionFactory  executor_factory.TransactionFactory
	taskQueueRepository asyncUploadTaskEnqueuer
	blobRepository      repositories.BlobRepository
	uploadLogRepository repositories.UploadLogRepository
	finalizer           asyncUploadFinalizer
	ingestionBucketUrl  string
}

func NewAsyncUploadWorker(
	transactionFactory executor_factory.TransactionFactory,
	taskQueueRepository asyncUploadTaskEnqueuer,
	blobRepository repositories.BlobRepository,
	uploadLogRepository repositories.UploadLogRepository,
	finalizer asyncUploadFinalizer,
	ingestionBucketUrl string,
) AsyncUploadWorker {
	return AsyncUploadWorker{
		transactionFactory:  transactionFactory,
		taskQueueRepository: taskQueueRepository,
		blobRepository:      blobRepository,
		uploadLogRepository: uploadLogRepository,
		finalizer:           finalizer,
		ingestionBucketUrl:  ingestionBucketUrl,
	}
}

func (w AsyncUploadWorker) Work(ctx context.Context, job *river.Job[models.AsyncUploadArgs]) error {
	// If we get an error retrieving the blob
	blobAttrs, err := w.blobRepository.GetBlobAttributes(ctx, w.ingestionBucketUrl, job.Args.Key)
	if err != nil {
		// If the blob was not uploaded yet
		if gcerrors.Code(err) == gcerrors.NotFound {
			// If we passed the token validity period, it won't be uploaded, so we cancel the job
			if time.Now().After(job.CreatedAt.Add(ASYNC_UPLOAD_TIMEOUT)) {
				if err := w.finalizer.FailUploadLog(
					ctx,
					job.Args.UploadLogId,
					models.IngestionFailureUploadNotReceived,
					"upload was not received before its deadline",
				); err != nil {
					return err
				}
				return river.JobCancel(
					errors.Newf("async upload: no file uploaded to blob storage after %s, cancelling watchdog", ASYNC_UPLOAD_TIMEOUT),
				)
			}

			// Otherwise, we snooze the job
			return river.JobSnooze(ASYNC_UPLOAD_TICK)
		}

		// Any other error fails the job immediately
		return err
	}

	if blobAttrs.Size > ASYNC_UPLOAD_MAX_SIZE {
		utils.LoggerFromContext(ctx).
			WarnContext(
				ctx, "uploaded file for async ingestion was too large",
				"org_id", job.Args.OrgId,
				"key", job.Args.Key,
				"size_GB", float64(blobAttrs.Size)/(1<<30),
			)

		if err := w.finalizer.FailUploadLog(
			ctx,
			job.Args.UploadLogId,
			models.IngestionFailureFileTooLarge,
			fmt.Sprintf("uploaded file exceeds the 10 GB limit (received %.2f GB)", float64(blobAttrs.Size)/(1<<30)),
		); err != nil {
			return err
		}
		if err := w.blobRepository.DeleteFile(ctx, w.ingestionBucketUrl, job.Args.Key); err != nil {
			utils.LoggerFromContext(ctx).
				ErrorContext(
					ctx, "could not delete uploaded file",
					"org_id", job.Args.OrgId,
					"key", job.Args.Key,
					"error", err.Error(),
				)
			return err
		}
		return nil
	}

	return w.transactionFactory.Transaction(ctx, func(tx repositories.Transaction) error {
		return w.taskQueueRepository.EnqueueCsvIngestionTask(ctx, tx, job.Args.OrgId, job.Args.UploadLogId, models.IngestionOptions{
			ShouldMonitor:          job.Args.IngestionOptions.ShouldMonitor,
			ContinuousScreeningIds: job.Args.IngestionOptions.ContinuousScreeningIds,
			ShouldScreen:           job.Args.IngestionOptions.ShouldScreen,
		})
	})
}
