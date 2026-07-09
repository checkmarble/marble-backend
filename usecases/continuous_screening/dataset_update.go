package continuous_screening

import (
	"context"

	"github.com/checkmarble/marble-backend/models"
	"github.com/google/uuid"
)

func (uc *ContinuousScreeningUsecase) ListContinuousScreeningDatasetUpdates(
	ctx context.Context,
	orgId uuid.UUID,
	pagination models.PaginationAndSorting,
) (models.Paginated[models.ContinuousScreeningDatasetUpdateEnriched], error) {
	if err := models.ValidatePagination(pagination); err != nil {
		return models.Paginated[models.ContinuousScreeningDatasetUpdateEnriched]{}, err
	}

	exec := uc.executorFactory.NewExecutor()

	org, err := uc.repository.GetOrganizationById(ctx, exec, orgId)
	if err != nil {
		return models.Paginated[models.ContinuousScreeningDatasetUpdateEnriched]{}, err
	}
	provider := org.GetScreeningProviderFor(models.ScreeningFeatureContinuousMonitoring)

	// Fetch one more item than requested so we can tell whether a next page exists,
	// then strip it back out below.
	limit := pagination.Limit
	pagination.Limit = limit + 1

	updates, err := uc.repository.ListContinuousScreeningDatasetUpdates(ctx, exec, orgId, pagination)
	if err != nil {
		return models.Paginated[models.ContinuousScreeningDatasetUpdateEnriched]{}, err
	}

	hasNextPage := len(updates) > limit
	updates = updates[:min(limit, len(updates))]

	// Overlay fresh data from the provider catalog onto each stored row.
	catalog, err := uc.screeningProvider.GetRawCatalog(ctx, provider)
	if err != nil {
		return models.Paginated[models.ContinuousScreeningDatasetUpdateEnriched]{}, err
	}
	current := make(map[string]struct{}, len(catalog.Current))
	for _, name := range catalog.Current {
		current[name] = struct{}{}
	}
	for i := range updates {
		if dataset, ok := catalog.Datasets[updates[i].DatasetName]; ok {
			updates[i].Title = dataset.Title
			updates[i].LiveVersion = dataset.Version
			_, updates[i].IsCurrent = current[updates[i].DatasetName]
		}
	}

	return models.Paginated[models.ContinuousScreeningDatasetUpdateEnriched]{
		Items:       updates,
		HasNextPage: hasNextPage,
	}, nil
}

func (uc *ContinuousScreeningUsecase) ListContinuousScreeningUpdateJobs(
	ctx context.Context,
	orgId uuid.UUID,
	pagination models.PaginationAndSorting,
) (models.Paginated[models.ContinuousScreeningUpdateJobSummary], error) {
	if err := models.ValidatePagination(pagination); err != nil {
		return models.Paginated[models.ContinuousScreeningUpdateJobSummary]{}, err
	}

	// Fetch one more item than requested so we can tell whether a next page exists,
	// then strip it back out below.
	limit := pagination.Limit
	pagination.Limit = limit + 1

	exec := uc.executorFactory.NewExecutor()
	jobs, err := uc.repository.ListContinuousScreeningUpdateJobs(ctx, exec, orgId, pagination)
	if err != nil {
		return models.Paginated[models.ContinuousScreeningUpdateJobSummary]{}, err
	}

	hasNextPage := len(jobs) > limit

	return models.Paginated[models.ContinuousScreeningUpdateJobSummary]{
		Items:       jobs[:min(limit, len(jobs))],
		HasNextPage: hasNextPage,
	}, nil
}

func (uc *ContinuousScreeningUsecase) ListContinuousScreeningClientDataIndexing(
	ctx context.Context,
	orgId uuid.UUID,
	pagination models.PaginationAndSorting,
) (models.Paginated[models.ContinuousScreeningClientDataIndexingSummary], error) {
	if err := models.ValidatePagination(pagination); err != nil {
		return models.Paginated[models.ContinuousScreeningClientDataIndexingSummary]{}, err
	}

	// Fetch one more item than requested so we can tell whether a next page exists,
	// then strip it back out below.
	limit := pagination.Limit
	pagination.Limit = limit + 1

	exec := uc.executorFactory.NewExecutor()
	items, err := uc.repository.ListContinuousScreeningClientDataIndexing(ctx, exec, orgId, pagination)
	if err != nil {
		return models.Paginated[models.ContinuousScreeningClientDataIndexingSummary]{}, err
	}

	hasNextPage := len(items) > limit

	return models.Paginated[models.ContinuousScreeningClientDataIndexingSummary]{
		Items:       items[:min(limit, len(items))],
		HasNextPage: hasNextPage,
	}, nil
}
