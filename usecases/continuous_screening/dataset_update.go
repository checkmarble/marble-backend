package continuous_screening

import (
	"context"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/google/uuid"
)

func (uc *ContinuousScreeningUsecase) ListContinuousScreeningDatasetUpdates(
	ctx context.Context,
	pagination models.PaginationAndSorting,
) (models.Paginated[models.ContinuousScreeningDatasetUpdateSummary], error) {
	if err := models.ValidatePagination(pagination); err != nil {
		return models.Paginated[models.ContinuousScreeningDatasetUpdateSummary]{}, err
	}

	// Fetch one more item than requested so we can tell whether a next page exists,
	// then strip it back out below.
	limit := pagination.Limit
	pagination.Limit = limit + 1

	exec := uc.executorFactory.NewExecutor()
	updates, err := uc.repository.ListContinuousScreeningDatasetUpdates(ctx, exec, pagination)
	if err != nil {
		return models.Paginated[models.ContinuousScreeningDatasetUpdateSummary]{}, err
	}

	hasNextPage := len(updates) > limit
	summaries := pure_utils.Map(updates[:min(limit, len(updates))],
		func(u models.ContinuousScreeningDatasetUpdate) models.ContinuousScreeningDatasetUpdateSummary {
			return models.ContinuousScreeningDatasetUpdateSummary{
				Id:          u.Id,
				DatasetName: u.DatasetName,
				Version:     u.Version,
				TotalItems:  u.TotalItems,
				CreatedAt:   u.CreatedAt,
			}
		})

	return models.Paginated[models.ContinuousScreeningDatasetUpdateSummary]{
		Items:       summaries,
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
