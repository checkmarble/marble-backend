package continuous_screening

import (
	"context"

	"github.com/checkmarble/marble-backend/models"
	"github.com/cockroachdb/errors"
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
		return models.Paginated[models.ContinuousScreeningDatasetUpdateEnriched]{},
			errors.Wrap(err, "failed to get organization for continuous screening dataset updates")
	}
	provider := org.GetScreeningProviderFor(models.ScreeningFeatureContinuousMonitoring)

	updates, err := listContinuousScreeningPage(
		pagination,
		func(pagination models.PaginationAndSorting) (
			[]models.ContinuousScreeningDatasetUpdateEnriched,
			error,
		) {
			return uc.repository.ListContinuousScreeningDatasetUpdates(
				ctx, exec, orgId, provider, pagination)
		},
	)
	if err != nil {
		return models.Paginated[models.ContinuousScreeningDatasetUpdateEnriched]{},
			errors.Wrap(err, "failed to list continuous screening dataset updates")
	}

	// Overlay fresh data from the provider catalog onto each stored row.
	catalog, err := uc.screeningProvider.GetRawCatalog(ctx, provider)
	if err != nil {
		return models.Paginated[models.ContinuousScreeningDatasetUpdateEnriched]{},
			errors.Wrap(err, "failed to get screening provider catalog")
	}
	current := make(map[string]struct{}, len(catalog.Current))
	for _, name := range catalog.Current {
		current[name] = struct{}{}
	}
	for i := range updates.Items {
		if dataset, ok := catalog.Datasets[updates.Items[i].DatasetName]; ok {
			updates.Items[i].Title = dataset.Title
			updates.Items[i].LiveVersion = dataset.Version
			_, updates.Items[i].IsCurrent = current[updates.Items[i].DatasetName]
		}
	}

	return updates, nil
}

func (uc *ContinuousScreeningUsecase) ListContinuousScreeningUpdateJobs(
	ctx context.Context,
	orgId uuid.UUID,
	pagination models.PaginationAndSorting,
) (models.Paginated[models.ContinuousScreeningUpdateJobSummary], error) {
	if err := models.ValidatePagination(pagination); err != nil {
		return models.Paginated[models.ContinuousScreeningUpdateJobSummary]{}, err
	}

	exec := uc.executorFactory.NewExecutor()

	org, err := uc.repository.GetOrganizationById(ctx, exec, orgId)
	if err != nil {
		return models.Paginated[models.ContinuousScreeningUpdateJobSummary]{},
			errors.Wrap(err, "failed to get organization for continuous screening update jobs")
	}
	provider := org.GetScreeningProviderFor(models.ScreeningFeatureContinuousMonitoring)

	jobs, err := listContinuousScreeningPage(
		pagination,
		func(pagination models.PaginationAndSorting) (
			[]models.ContinuousScreeningUpdateJobSummary,
			error,
		) {
			return uc.repository.ListContinuousScreeningUpdateJobs(
				ctx, exec, orgId, provider, pagination)
		},
	)
	if err != nil {
		return models.Paginated[models.ContinuousScreeningUpdateJobSummary]{},
			errors.Wrap(err, "failed to list continuous screening update jobs")
	}

	return jobs, nil
}

func listContinuousScreeningPage[T any](
	pagination models.PaginationAndSorting,
	list func(models.PaginationAndSorting) ([]T, error),
) (models.Paginated[T], error) {
	// Fetch one more item than requested so we can tell whether a next page exists.
	limit := pagination.Limit
	pagination.Limit = limit + 1

	items, err := list(pagination)
	if err != nil {
		return models.Paginated[T]{}, err
	}

	return models.Paginated[T]{
		Items:       items[:min(limit, len(items))],
		HasNextPage: len(items) > limit,
	}, nil
}

func (uc *ContinuousScreeningUsecase) ListContinuousScreeningClientDataIndexing(
	ctx context.Context,
	orgId uuid.UUID,
	pagination models.PaginationAndSorting,
) (models.ContinuousScreeningClientDataIndexing, error) {
	if err := models.ValidatePagination(pagination); err != nil {
		return models.ContinuousScreeningClientDataIndexing{}, err
	}

	// Fetch one more item than requested so we can tell whether a next page exists,
	// then strip it back out below.
	limit := pagination.Limit
	pagination.Limit = limit + 1

	exec := uc.executorFactory.NewExecutor()

	org, err := uc.repository.GetOrganizationById(ctx, exec, orgId)
	if err != nil {
		return models.ContinuousScreeningClientDataIndexing{},
			errors.Wrap(err, "failed to get organization for continuous screening client data indexing")
	}

	provider := org.GetScreeningProviderFor(models.ScreeningFeatureContinuousMonitoring)
	catalog, err := uc.screeningProvider.GetRawCatalog(ctx, provider)
	if err != nil {
		return models.ContinuousScreeningClientDataIndexing{},
			errors.Wrap(err, "failed to get screening provider catalog")
	}

	dataset, datasetFound := catalog.Datasets[orgCustomDatasetName(orgId)]
	var indexVersion *string
	if datasetFound {
		indexVersion = dataset.IndexVersion
	}

	indexing, err := uc.repository.ListContinuousScreeningClientDataIndexing(
		ctx, exec, orgId, indexVersion, pagination)
	if err != nil {
		return models.ContinuousScreeningClientDataIndexing{},
			errors.Wrap(err, "failed to list continuous screening client data indexing")
	}

	hasNextPage := len(indexing.Items.Items) > limit

	indexing.Items = models.Paginated[models.ContinuousScreeningClientDataIndexingSummary]{
		Items:       indexing.Items.Items[:min(limit, len(indexing.Items.Items))],
		HasNextPage: hasNextPage,
	}
	if datasetFound {
		indexing.Version = dataset.Version
		indexing.IndexVersion = dataset.IndexVersion
		indexing.IndexCurrent = dataset.IndexCurrent
	}

	return indexing, nil
}
