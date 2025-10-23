package screening_monitoring

import (
	"context"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/checkmarble/marble-backend/usecases/security"
	"github.com/google/uuid"
)

type ScreeningMonitoringUsecaseRepository interface {
	GetScreeningMonitoringConfig(
		ctx context.Context,
		exec repositories.Executor,
		Id uuid.UUID,
	) (
		models.ScreeningMonitoringConfig, error)
	GetScreeningMonitoringConfigsByOrgId(
		ctx context.Context,
		exec repositories.Executor,
		orgId string,
	) (
		[]models.ScreeningMonitoringConfig, error)
	CreateScreeningMonitoringConfig(
		ctx context.Context,
		exec repositories.Executor,
		input models.CreateScreeningMonitoringConfig,
	) (models.ScreeningMonitoringConfig, error)
	UpdateScreeningMonitoringConfig(
		ctx context.Context,
		exec repositories.Executor,
		id uuid.UUID,
		input models.UpdateScreeningMonitoringConfig,
	) (models.ScreeningMonitoringConfig, error)
}

type ScreeningMonitoringUsecase struct {
	executorFactory executor_factory.ExecutorFactory

	enforceSecurity               security.EnforceSecurityScreeningMonitoring
	screeningMonitoringRepository ScreeningMonitoringUsecaseRepository
}

func NewScreeningMonitoringUsecase(
	executorFactory executor_factory.ExecutorFactory,
	enforceSecurity security.EnforceSecurityScreeningMonitoring,
	screeningMonitoringRepository ScreeningMonitoringUsecaseRepository,
) ScreeningMonitoringUsecase {
	return ScreeningMonitoringUsecase{
		executorFactory:               executorFactory,
		enforceSecurity:               enforceSecurity,
		screeningMonitoringRepository: screeningMonitoringRepository,
	}
}
