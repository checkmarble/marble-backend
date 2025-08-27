package scheduled_execution

import (
	"context"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/cockroachdb/errors"
	"github.com/riverqueue/river"
)

type OpenSanctionsProvider interface {
	IsConfigured(context.Context) (bool, error)
	IsSelfHosted(context.Context) bool
}

type ScreeningRepository interface {
	GetScreening(ctx context.Context, exec repositories.Executor, id string) (models.ScreeningWithMatches, error)
}

type ScreeningUsecase interface {
	EnrichMatchWithoutAuthorization(ctx context.Context, matchId string) (models.ScreeningMatch, error)
}

type MatchEnrichmentWorker struct {
	river.WorkerDefaults[models.MatchEnrichmentArgs]

	executorFactory     executor_factory.ExecutorFactory
	openSanctionsConfig OpenSanctionsProvider
	screeningUsecase    ScreeningUsecase
	repository          ScreeningRepository
}

func NewMatchEnrichmentWorker(
	executorFactory executor_factory.ExecutorFactory,
	openSanctionsProvider OpenSanctionsProvider,
	screeningUsecase ScreeningUsecase,
	repository ScreeningRepository,
) MatchEnrichmentWorker {
	return MatchEnrichmentWorker{
		executorFactory:     executorFactory,
		openSanctionsConfig: openSanctionsProvider,
		screeningUsecase:    screeningUsecase,
		repository:          repository,
	}
}

func (w *MatchEnrichmentWorker) Work(ctx context.Context, job *river.Job[models.MatchEnrichmentArgs]) error {
	logger := utils.LoggerFromContext(ctx)

	if ok, err := w.openSanctionsConfig.IsConfigured(ctx); err != nil || !ok {
		logger.WarnContext(ctx, "MatchEnrichmentWorker: Open Sanctions provider not configured, aborting...")
		return nil
	}
	if !w.openSanctionsConfig.IsSelfHosted(ctx) {
		logger.WarnContext(ctx, "MatchEnrichmentWorker: Open Sanctions provider is not self-hosted, aborting...")
		return nil
	}

	var errs error

	scc, err := w.repository.GetScreening(ctx, w.executorFactory.NewExecutor(), job.Args.ScreeningId)
	if err != nil {
		return err
	}

	for _, match := range scc.Matches {
		if match.Enriched {
			continue
		}

		if _, err := w.screeningUsecase.EnrichMatchWithoutAuthorization(ctx, match.Id); err != nil {
			errs = errors.Join(errs, err)
		}
	}

	return errs
}

func (w *MatchEnrichmentWorker) Timeout(job *river.Job[models.MatchEnrichmentArgs]) time.Duration {
	return 10 * time.Second
}
