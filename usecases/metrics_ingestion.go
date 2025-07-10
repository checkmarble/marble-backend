package usecases

import (
	"context"
	"fmt"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/cockroachdb/errors"
)

type MetricsIngestionRepository interface {
	SendMetrics(ctx context.Context, collection models.MetricsCollection) error
}

type MetricsIngestionUsecase struct {
	metricRepository  MetricsIngestionRepository
	licenseRepository publicLicenseRepository
	executorFactory   executor_factory.ExecutorFactory
}

func NewMetricsIngestionUsecase(
	metricRepository MetricsIngestionRepository,
	licenseRepository publicLicenseRepository,
	executorFactory executor_factory.ExecutorFactory,
) MetricsIngestionUsecase {
	return MetricsIngestionUsecase{
		metricRepository:  metricRepository,
		licenseRepository: licenseRepository,
		executorFactory:   executorFactory,
	}
}

func (u *MetricsIngestionUsecase) IngestMetrics(ctx context.Context, collection models.MetricsCollection) error {
	logger := utils.LoggerFromContext(ctx)

	if collection.LicenseKey != nil {
		logger.DebugContext(ctx, "Checking license")
		license, err := u.validateLicense(ctx, *collection.LicenseKey)
		if err != nil {
			logger.ErrorContext(ctx, "Failed to validate license", "error", err.Error())
			return errors.Wrap(models.UnAuthorizedError, "invalid license")
		}
		collection.LicenseName = &license.OrganizationName
	}

	err := u.metricRepository.SendMetrics(ctx, collection)
	if err != nil {
		logger.ErrorContext(ctx, "Failed to send metrics to BigQuery", "error", err.Error())
		return fmt.Errorf("failed to send metrics to BigQuery: %w", err)
	}

	return nil
}

// Only check if the license exists
func (u *MetricsIngestionUsecase) validateLicense(ctx context.Context, licenseKey string) (models.License, error) {
	logger := utils.LoggerFromContext(ctx)

	license, err := u.licenseRepository.GetLicenseByKey(ctx,
		u.executorFactory.NewExecutor(), licenseKey)
	if err != nil {
		if !errors.Is(err, models.NotFoundError) {
			utils.LogAndReportSentryError(ctx, err)
		}

		logger.WarnContext(ctx, "Invalid license", "license", licenseKey)
		return models.License{}, errors.Wrap(models.UnAuthorizedError, "invalid license")
	}
	return license, nil
}
