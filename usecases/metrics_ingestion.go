package usecases

import (
	"context"

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
		license, err := u.licenseRepository.GetLicenseByKey(ctx,
			u.executorFactory.NewExecutor(), *collection.LicenseKey)
		if err != nil {
			if !errors.Is(err, models.NotFoundError) {
				return errors.Wrap(err, "Error fetching license")
			}
			logger.DebugContext(ctx, "License not found, don't link to license")
			collection.LicenseKey = nil
			collection.LicenseName = nil
		} else {
			collection.LicenseName = &license.OrganizationName
		}
	}

	err := u.metricRepository.SendMetrics(ctx, collection)
	if err != nil {
		return err
	}

	return nil
}
