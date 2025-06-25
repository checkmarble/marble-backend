package usecases

import (
	"context"

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

func (u *LivenessUsecase) Liveness(ctx context.Context) error {
	return u.livenessRepository.Liveness(ctx, u.executorFactory.NewExecutor())
}
