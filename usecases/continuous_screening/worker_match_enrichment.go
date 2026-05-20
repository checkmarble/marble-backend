package continuous_screening

import (
	"context"
	"encoding/json"
	"maps"
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
	IsConfigured(context.Context, models.ScreeningProvider) (bool, error)
	IsSelfHosted(context.Context) bool
	EnrichMatch(ctx context.Context, providerName models.ScreeningProvider, match models.ScreeningMatch) ([]byte, error)
}

type matchEnrichmentWorkerRepository interface {
	GetOrganizationById(ctx context.Context, exec repositories.Executor, organizationId uuid.UUID) (models.Organization, error)

	GetContinuousScreeningWithMatchesById(
		ctx context.Context,
		exec repositories.Executor,
		id uuid.UUID,
	) (models.ContinuousScreeningWithMatches, error)
	GetContinuousScreeningMatch(
		ctx context.Context,
		exec repositories.Executor,
		id uuid.UUID,
	) (models.ContinuousScreeningMatch, error)
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

type ContinuousScreeningMatchEnrichmentWorker struct {
	river.WorkerDefaults[models.ContinuousScreeningMatchEnrichmentArgs]

	executorFactory     executor_factory.ExecutorFactory
	openSanctionsConfig matchEnrichmentWorkerOpenSanctionsProvider
	repository          matchEnrichmentWorkerRepository
}

func NewContinuousScreeningMatchEnrichmentWorker(
	executorFactory executor_factory.ExecutorFactory,
	openSanctionsProvider matchEnrichmentWorkerOpenSanctionsProvider,
	repository matchEnrichmentWorkerRepository,
) *ContinuousScreeningMatchEnrichmentWorker {
	return &ContinuousScreeningMatchEnrichmentWorker{
		executorFactory:     executorFactory,
		openSanctionsConfig: openSanctionsProvider,
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

	exec := w.executorFactory.NewExecutor()

	continuousScreeningWithMatches, err := w.repository.GetContinuousScreeningWithMatchesById(
		ctx,
		exec,
		job.Args.ContinuousScreeningId,
	)
	if err != nil {
		return err
	}

	org, err := w.repository.GetOrganizationById(ctx, exec, continuousScreeningWithMatches.OrgId)
	if err != nil {
		return errors.Wrap(err, "could not retrieve organization")
	}
	provider := org.GetScreeningProviderFor(models.ScreeningFeatureContinuousMonitoring)

	if ok, err := w.openSanctionsConfig.IsConfigured(ctx, provider); err != nil || !ok {
		logger.WarnContext(ctx, "ContinuousScreeningMatchEnrichmentWorker: Open Sanctions provider not configured, aborting...")
		return nil
	}
	if !w.openSanctionsConfig.IsSelfHosted(ctx) {
		logger.WarnContext(ctx, "ContinuousScreeningMatchEnrichmentWorker: Open Sanctions provider is not self-hosted, aborting...")
		return nil
	}

	var errs error

	// For DatasetTriggered screenings:
	// - Enrich the OpenSanctions entity (external data from dataset)
	// - Don't enrich matches (they are organization's own data)
	if continuousScreeningWithMatches.IsDatasetTriggered() {
		if !continuousScreeningWithMatches.OpenSanctionEntityEnriched &&
			continuousScreeningWithMatches.OpenSanctionEntityId != nil {
			if err := w.enrichEntity(ctx, provider, continuousScreeningWithMatches); err != nil {
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

			if err := w.enrichMatch(ctx, provider, match); err != nil {
				errs = errors.Join(errs, err)
			}
		}
	}

	return errs
}

func (w *ContinuousScreeningMatchEnrichmentWorker) enrichEntity(
	ctx context.Context,
	providerName models.ScreeningProvider,
	screening models.ContinuousScreeningWithMatches,
) error {
	if screening.OpenSanctionEntityEnriched {
		return errors.Wrap(models.UnprocessableEntityError,
			"this continuous screening entity was already enriched")
	}

	if screening.OpenSanctionEntityId == nil {
		return errors.Wrap(models.BadParameterError,
			"this continuous screening has no OpenSanction entity to enrich")
	}

	// Create a fake match to use the EnrichMatch method from OpenSanctions repository
	fakeMatch := models.ScreeningMatch{
		EntityId: *screening.OpenSanctionEntityId,
	}

	newPayload, err := w.openSanctionsConfig.EnrichMatch(ctx, providerName, fakeMatch)
	if err != nil {
		return err
	}

	mergedPayload, err := mergePayloads(screening.OpenSanctionEntityPayload, newPayload)
	if err != nil {
		return errors.Wrap(err,
			"could not merge payloads for continuous screening entity enrichment")
	}

	exec := w.executorFactory.NewExecutor()
	if err := w.repository.UpdateContinuousScreeningEntityEnrichedPayload(
		ctx, exec, screening.Id, mergedPayload,
	); err != nil {
		return err
	}

	return nil
}

func (w *ContinuousScreeningMatchEnrichmentWorker) enrichMatch(
	ctx context.Context,
	providerName models.ScreeningProvider,
	match models.ContinuousScreeningMatch,
) error {
	if match.Enriched {
		utils.LoggerFromContext(ctx).DebugContext(ctx,
			"continuous screening match already enriched, skipping",
			"match_id", match.Id,
		)
		return nil
	}

	// Create a fake screening match to use the EnrichMatch method from OpenSanctions repository
	fakeMatch := models.ScreeningMatch{
		EntityId: match.OpenSanctionEntityId,
	}

	newPayload, err := w.openSanctionsConfig.EnrichMatch(ctx, providerName, fakeMatch)
	if err != nil {
		return err
	}

	mergedPayload, err := mergePayloads(match.Payload, newPayload)
	if err != nil {
		return errors.Wrap(err,
			"could not merge payloads for continuous screening match enrichment")
	}

	exec := w.executorFactory.NewExecutor()
	if err := w.repository.UpdateContinuousScreeningMatchEnrichedPayload(
		ctx, exec, match.Id, mergedPayload,
	); err != nil {
		return err
	}

	return nil
}

func mergePayloads(originalRaw, newRaw []byte) ([]byte, error) {
	var original, new map[string]any

	if err := json.Unmarshal(originalRaw, &original); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(newRaw, &new); err != nil {
		return nil, err
	}

	maps.Copy(original, new)

	out, err := json.Marshal(original)
	if err != nil {
		return nil, err
	}

	return out, nil
}
