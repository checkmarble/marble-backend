package usecases

import (
	"context"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
)

type healthRepository interface {
	Liveness(ctx context.Context, exec repositories.Executor) error
}

type OpenSanctionsHealthRepository interface {
	IsConfigured(ctx context.Context) (bool, error)
}

type HealthUsecase struct {
	executorFactory  executor_factory.ExecutorFactory
	healthRepository healthRepository

	openSanctionsRepository OpenSanctionsHealthRepository
	hasOpensanctionsSetup   bool
}

func (u *HealthUsecase) GetHealthStatus(ctx context.Context) models.HealthStatus {
	statuses := []models.HealthItemStatus{}

	// Check database health
	err := u.healthRepository.Liveness(ctx, u.executorFactory.NewExecutor())
	statuses = append(statuses, models.HealthItemStatus{
		Name:   models.DatabaseHealthItemName,
		Status: err == nil,
	})

	// Check Open Sanctions health
	if u.hasOpensanctionsSetup {
		ok, err := u.openSanctionsRepository.IsConfigured(ctx)
		statuses = append(statuses, models.HealthItemStatus{
			Name:   models.OpenSanctionsHealthItemName,
			Status: ok && err == nil,
		})
	}

	return models.HealthStatus{
		Statuses: statuses,
	}
}
