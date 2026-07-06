package continuous_screening

import (
	"context"

	"github.com/checkmarble/marble-backend/models"
	"github.com/google/uuid"
)

func (uc *ContinuousScreeningUsecase) ListContinuousScreeningDatasetUpdates(
	ctx context.Context,
	pagination models.PaginationAndSorting,
) (models.Paginated[models.ContinuousScreeningDatasetUpdate], error) {
	if err := models.ValidatePagination(pagination); err != nil {
		return models.Paginated[models.ContinuousScreeningDatasetUpdate]{}, err
	}

	// Fetch one more item than requested so we can tell whether a next page exists,
	// then strip it back out below.
	limit := pagination.Limit
	pagination.Limit = limit + 1

	exec := uc.executorFactory.NewExecutor()
	updates, err := uc.repository.ListContinuousScreeningDatasetUpdates(ctx, exec, pagination)
	if err != nil {
		return models.Paginated[models.ContinuousScreeningDatasetUpdate]{}, err
	}

	hasNextPage := len(updates) > limit

	return models.Paginated[models.ContinuousScreeningDatasetUpdate]{
		Items:       updates[:min(limit, len(updates))],
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
