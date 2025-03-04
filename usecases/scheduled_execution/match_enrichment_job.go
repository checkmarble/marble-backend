package scheduled_execution

import (
	"context"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/cockroachdb/errors"
	"github.com/riverqueue/river"
)

type SanctionCheckRepository interface {
	GetSanctionCheck(ctx context.Context, exec repositories.Executor, id string) (models.SanctionCheckWithMatches, error)
}

type SanctionCheckUsecase interface {
	EnrichMatchWithoutAuthorization(ctx context.Context, matchId string) (models.SanctionCheckMatch, error)
}

type MatchEnrichmentWorker struct {
	river.WorkerDefaults[models.MatchEnrichmentArgs]

	executorFactory      executor_factory.ExecutorFactory
	sanctionCheckUsecase SanctionCheckUsecase
	repository           SanctionCheckRepository
}

func NewMatchEnrichmentWorker(
	executorFactory executor_factory.ExecutorFactory,
	sanctionCheckUsecase SanctionCheckUsecase,
	repository SanctionCheckRepository,
) MatchEnrichmentWorker {
	return MatchEnrichmentWorker{
		executorFactory:      executorFactory,
		sanctionCheckUsecase: sanctionCheckUsecase,
		repository:           repository,
	}
}

func (w *MatchEnrichmentWorker) Work(ctx context.Context, job *river.Job[models.MatchEnrichmentArgs]) error {
	var errs error

	scc, err := w.repository.GetSanctionCheck(ctx, w.executorFactory.NewExecutor(), job.Args.SanctionCheckId)
	if err != nil {
		return err
	}

	for _, match := range scc.Matches {
		if match.Enriched {
			continue
		}

		if _, err := w.sanctionCheckUsecase.EnrichMatchWithoutAuthorization(ctx, match.Id); err != nil {
			errs = errors.Join(errs, err)
		}
	}

	return errs
}
