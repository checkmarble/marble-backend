package usecases

import (
	"context"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/checkmarble/marble-backend/usecases/security"
	"github.com/cockroachdb/errors"
)

type dashboardRepository interface {
	Dashboard(ctx context.Context, exec repositories.Executor, months int) (models.Dashboard, error)
}

type DashboardUsecase struct {
	enforceSecurity security.EnforceSecurityDashboard
	executorFactory executor_factory.ExecutorFactory
	repository      dashboardRepository
}

func NewDashboardUsecase(enforceSecurity security.EnforceSecurityDashboard, executorFactory executor_factory.ExecutorFactory, repository dashboardRepository) DashboardUsecase {
	return DashboardUsecase{
		enforceSecurity: enforceSecurity,
		executorFactory: executorFactory,
		repository:      repository,
	}
}

func (uc DashboardUsecase) Get(ctx context.Context, months int) (models.Dashboard, error) {
	if err := uc.enforceSecurity.ReadDashboard(); err != nil {
		return models.Dashboard{}, err
	}

	switch months {
	case 1, 3, 6, 12:
	default:
		return models.Dashboard{}, errors.Wrap(models.BadParameterError, "months must be 1, 3, 6, or 12")
	}

	return uc.repository.Dashboard(ctx, uc.executorFactory.NewExecutor(), months)
}
