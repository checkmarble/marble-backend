package usecases

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/checkmarble/marble-backend/infra"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/checkmarble/marble-backend/usecases/worker_jobs"
	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
)

func (usecase *IngestionUseCase) GenerateUploadLink(
	ctx context.Context,
	orgId uuid.UUID, recordType string,
	ingestionOptions models.IngestionOptions,
	headers http.Header,
) (string, error) {
	if err := usecase.validateAsyncIngestionHeaders(headers); err != nil {
		return "", err
	}

	uploadId := pure_utils.NewId()
	key := fmt.Sprintf("uploads/%s/%s/%s", orgId, recordType, uploadId)

	if err := usecase.enforceSecurity.CanIngest(orgId); err != nil {
		return "", err
	}

	exec := usecase.executorFactory.NewExecutor()

	org, err := usecase.continuousScreeningRepository.GetOrganizationById(ctx, exec, orgId)
	if err != nil {
		return "", errors.Wrap(err, "error getting organization")
	}

	dataModel, err := usecase.dataModelRepository.GetDataModel(ctx, exec, orgId, false, true)
	if err != nil {
		return "", errors.Wrap(err, "error getting data model in IngestObject")
	}

	if _, ok := dataModel.Tables[recordType]; !ok {
		return "", errors.WithDetailf(
			models.NotFoundError,
			"table %s not found in data model in IngestObject", recordType,
		)
	}

	if ingestionOptions.ShouldMonitor {
		continuousScreeningConfigs, err := usecase.continuousScreeningRepository.ListContinuousScreeningConfigByStableIds(
			ctx, exec, orgId, org.GetScreeningProviderFor(models.ScreeningFeatureContinuousMonitoring), ingestionOptions.ContinuousScreeningIds)
		if err != nil {
			return "", err
		}

		if err := validateContinuousScreeningConfigs(continuousScreeningConfigs, ingestionOptions.ContinuousScreeningIds, recordType); err != nil {
			return "", err
		}
	}

	return executor_factory.TransactionReturnValue(ctx, usecase.transactionFactory, func(tx repositories.Transaction) (string, error) {
		if err := usecase.taskEnqueuer.EnqueueAsyncUploadTask(ctx, tx, orgId, recordType, key, ingestionOptions); err != nil {
			return "", err
		}

		hostOverride := infra.GetLocalCdnDomain()

		return usecase.blobRepository.GenerateWriteSignedUrl(ctx, usecase.ingestionBucketUrl, key, worker_jobs.ASYNC_UPLOAD_START_TIMEOUT, hostOverride)
	})
}

func (usecase *IngestionUseCase) validateAsyncIngestionHeaders(headers http.Header) error {
	if headers.Get("content-type") != "text/csv" {
		return errors.WithDetail(models.BadParameterError, "content-type header must be `text/csv`")
	}

	switch {
	case strings.HasPrefix(usecase.ingestionBucketUrl, "gs://"):
		if headers.Get("x-goog-if-generation-match") != "0" {
			return errors.WithDetail(models.BadParameterError, "x-goog-if-generation-match header must be set to `0`")
		}

	case strings.HasPrefix(usecase.ingestionBucketUrl, "s3://"):
		if headers.Get("if-none-match") != "*" {
			return errors.WithDetail(models.BadParameterError, "if-none-match header must be set to `*`")
		}

	case strings.HasPrefix(usecase.ingestionBucketUrl, "azblob://"):
		if headers.Get("x-ms-blob-type") != "BlockBlob" {
			return errors.WithDetail(models.BadParameterError, "x-ms-blob-type header must be set to `BlockBlob`")
		}
	}

	return nil
}
