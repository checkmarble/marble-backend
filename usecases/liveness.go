package usecases

import (
	"context"

	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
)

type livenessRepository interface {
	Liveness(ctx context.Context, exec repositories.Executor) error
}

type LivenessUsecase struct {
	executorFactory    executor_factory.ExecutorFactory
	livenessRepository livenessRepository
}

func (u *LivenessUsecase) Liveness(ctx context.Context) error {
	return u.livenessRepository.Liveness(ctx, u.executorFactory.NewExecutor())
}
