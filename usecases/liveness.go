package usecases

import (
	"context"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
)

type livenessRepository interface {
	Liveness(ctx context.Context, exec repositories.Executor) error
}

type OpenSanctionsRepository interface {
	IsConfigured(ctx context.Context) (bool, error)
}

type LivenessUsecase struct {
	// For database healthcheck
	executorFactory    executor_factory.ExecutorFactory
	livenessRepository livenessRepository

	// For Open Sanctions healthcheck
	openSanctionsRepository OpenSanctionsRepository
	hasOpensanctionsSetup   bool
}

func (u *LivenessUsecase) Liveness(ctx context.Context) models.LivenessStatus {
	statuses := []models.LivenessItemStatus{}

	// Check database health
	if err := u.livenessRepository.Liveness(ctx, u.executorFactory.NewExecutor()); err != nil {
		statuses = append(statuses, models.LivenessItemStatus{
			Name:   models.DatabaseLivenessItemName,
			IsLive: false,
			Error:  err,
		})
	} else {
		statuses = append(statuses, models.LivenessItemStatus{
			Name:   models.DatabaseLivenessItemName,
			IsLive: true,
			Error:  nil,
		})
	}

	// Check Open Sanctions health
	if u.hasOpensanctionsSetup {
		if ok, err := u.openSanctionsRepository.IsConfigured(ctx); err != nil || !ok {
			statuses = append(statuses, models.LivenessItemStatus{
				Name:   models.OpenSanctionsLivenessItemName,
				IsLive: false,
				Error:  err,
			})
		} else {
			statuses = append(statuses, models.LivenessItemStatus{
				Name:   models.OpenSanctionsLivenessItemName,
				IsLive: true,
				Error:  nil,
			})
		}
	}

	return models.LivenessStatus{
		Statuses: statuses,
	}
}
