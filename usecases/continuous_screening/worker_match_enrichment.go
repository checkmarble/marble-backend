package continuous_screening

import (
	"context"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
	"github.com/riverqueue/river"
)

type matchEnrichmentWorkerOpenSanctionsProvider interface {
	IsConfigured(context.Context) (bool, error)
	IsSelfHosted(context.Context) bool
}

type matchEnrichmentWorkerRepository interface {
	GetContinuousScreeningWithMatchesById(
		ctx context.Context,
		exec repositories.Executor,
		id uuid.UUID,
	) (models.ContinuousScreeningWithMatches, error)
	UpdateContinuousScreeningEntityEnrichedPayload(
		ctx context.Context,
		exec repositories.Executor,
		id uuid.UUID,
		enrichedPayload []byte,
	) error
	UpdateContinuousScreeningMatchEnrichedPayload(
		ctx context.Context,
		exec repositories.Executor,
		id uuid.UUID,
		enrichedPayload []byte,
	) error
}

type matchEnrichmentWorkerUsecase interface {
	EnrichContinuousScreeningEntityWithoutAuthorization(
		ctx context.Context,
		continuousScreeningId uuid.UUID,
	) error
	EnrichContinuousScreeningMatchWithoutAuthorization(
		ctx context.Context,
		matchId uuid.UUID,
	) error
}

type ContinuousScreeningMatchEnrichmentWorker struct {
	river.WorkerDefaults[models.ContinuousScreeningMatchEnrichmentArgs]

	executorFactory     executor_factory.ExecutorFactory
	openSanctionsConfig matchEnrichmentWorkerOpenSanctionsProvider
	usecase             matchEnrichmentWorkerUsecase
	repository          matchEnrichmentWorkerRepository
}

func NewContinuousScreeningMatchEnrichmentWorker(
	executorFactory executor_factory.ExecutorFactory,
	openSanctionsProvider matchEnrichmentWorkerOpenSanctionsProvider,
	usecase matchEnrichmentWorkerUsecase,
	repository matchEnrichmentWorkerRepository,
) *ContinuousScreeningMatchEnrichmentWorker {
	return &ContinuousScreeningMatchEnrichmentWorker{
		executorFactory:     executorFactory,
		openSanctionsConfig: openSanctionsProvider,
		usecase:             usecase,
		repository:          repository,
	}
}

func (w *ContinuousScreeningMatchEnrichmentWorker) Timeout(
	job *river.Job[models.ContinuousScreeningMatchEnrichmentArgs],
) time.Duration {
	return 10 * time.Second
}

func (w *ContinuousScreeningMatchEnrichmentWorker) Work(
	ctx context.Context,
	job *river.Job[models.ContinuousScreeningMatchEnrichmentArgs],
) error {
	logger := utils.LoggerFromContext(ctx)

	if ok, err := w.openSanctionsConfig.IsConfigured(ctx); err != nil || !ok {
		logger.WarnContext(ctx, "ContinuousScreeningMatchEnrichmentWorker: Open Sanctions provider not configured, aborting...")
		return nil
	}
	if !w.openSanctionsConfig.IsSelfHosted(ctx) {
		logger.WarnContext(ctx, "ContinuousScreeningMatchEnrichmentWorker: Open Sanctions provider is not self-hosted, aborting...")
		return nil
	}

	var errs error

	continuousScreeningWithMatches, err := w.repository.GetContinuousScreeningWithMatchesById(
		ctx,
		w.executorFactory.NewExecutor(),
		job.Args.ContinuousScreeningId,
	)
	if err != nil {
		return err
	}

	// For DatasetTriggered screenings:
	// - Enrich the OpenSanctions entity (external data from dataset)
	// - Don't enrich matches (they are organization's own data)
	if continuousScreeningWithMatches.IsDatasetTriggered() {
		if !continuousScreeningWithMatches.OpenSanctionEntityEnriched &&
			continuousScreeningWithMatches.OpenSanctionEntityId != nil {
			if err := w.usecase.EnrichContinuousScreeningEntityWithoutAuthorization(
				ctx,
				continuousScreeningWithMatches.Id,
			); err != nil {
				errs = errors.Join(errs, err)
			}
		}
	} else {
		// For ObjectTriggered screenings:
		// - Don't enrich the screening entity (it's organization's own data)
		// - Enrich the matches (they are OpenSanctions entities)
		for _, match := range continuousScreeningWithMatches.Matches {
			if match.Enriched {
				continue
			}

			if err := w.usecase.EnrichContinuousScreeningMatchWithoutAuthorization(ctx, match.Id); err != nil {
				errs = errors.Join(errs, err)
			}
		}
	}

	return errs
}
