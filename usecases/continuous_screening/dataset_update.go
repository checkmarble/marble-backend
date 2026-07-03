package continuous_screening

import (
	"context"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
)

func (uc *ContinuousScreeningUsecase) ListContinuousScreeningDatasetUpdates(
	ctx context.Context,
	pagination models.PaginationAndSorting,
) ([]models.ContinuousScreeningDatasetUpdateSummary, error) {
	if err := models.ValidatePagination(pagination); err != nil {
		return nil, err
	}

	exec := uc.executorFactory.NewExecutor()
	updates, err := uc.repository.ListContinuousScreeningDatasetUpdates(ctx, exec, pagination)
	if err != nil {
		return nil, err
	}

	return pure_utils.Map(updates,
		func(u models.ContinuousScreeningDatasetUpdate) models.ContinuousScreeningDatasetUpdateSummary {
			return models.ContinuousScreeningDatasetUpdateSummary{
				Id:          u.Id,
				DatasetName: u.DatasetName,
				Version:     u.Version,
				TotalItems:  u.TotalItems,
				CreatedAt:   u.CreatedAt,
			}
		}), nil
}
