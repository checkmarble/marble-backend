package usecases

import (
	"context"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/checkmarble/marble-backend/usecases/security"
	"github.com/checkmarble/marble-backend/usecases/tracking"

	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
)

type FeatureUseCaseRepository interface {
	ListFeatures(ctx context.Context, exec repositories.Executor) ([]models.Feature, error)
	CreateFeature(ctx context.Context, exec repositories.Executor,
		attributes models.CreateFeatureAttributes, newFeatureId string) error
	UpdateFeature(ctx context.Context, exec repositories.Executor,
		attributes models.UpdateFeatureAttributes) error
	GetFeatureById(ctx context.Context, exec repositories.Executor, featureId string) (models.Feature, error)
	SoftDeleteFeature(ctx context.Context, exec repositories.Executor, featureId string) error
}

type FeatureUseCase struct {
	enforceSecurity    security.EnforceSecurityFeatures
	transactionFactory executor_factory.TransactionFactory
	executorFactory    executor_factory.ExecutorFactory
	repository         FeatureUseCaseRepository
}

func (usecase *FeatureUseCase) ListAllFeatures(ctx context.Context) ([]models.Feature, error) {
	features, err := usecase.repository.ListFeatures(
		ctx,
		usecase.executorFactory.NewExecutor())
	if err != nil {
		return nil, err
	}

	for _, t := range features {
		if err := usecase.enforceSecurity.ReadFeature(t); err != nil {
			return nil, err
		}
	}
	return features, err
}

func (usecase *FeatureUseCase) CreateFeature(ctx context.Context,
	attributes models.CreateFeatureAttributes,
) (models.Feature, error) {
	if err := usecase.enforceSecurity.CreateFeature(); err != nil {
		return models.Feature{}, err
	}

	feature, err := executor_factory.TransactionReturnValue(ctx,
		usecase.transactionFactory, func(tx repositories.Transaction) (models.Feature, error) {
			newFeatureId := uuid.NewString()
			if err := usecase.repository.CreateFeature(ctx, tx, attributes, newFeatureId); err != nil {
				if repositories.IsUniqueViolationError(err) {
					return models.Feature{}, errors.Wrap(models.ConflictError,
						"There is already a feature by this name")
				}
				return models.Feature{}, err
			}
			return usecase.repository.GetFeatureById(ctx, tx, newFeatureId)
		})
	if err != nil {
		return models.Feature{}, err
	}

	tracking.TrackEvent(ctx, models.AnalyticsFeatureCreated, map[string]interface{}{
		"feature_id": feature.Id,
	})

	return feature, err
}

func (usecase *FeatureUseCase) GetFeatureById(ctx context.Context, featureId string) (models.Feature, error) {
	t, err := usecase.repository.GetFeatureById(ctx, usecase.executorFactory.NewExecutor(), featureId)
	if err != nil {
		return models.Feature{}, err
	}
	if err := usecase.enforceSecurity.ReadFeature(t); err != nil {
		return models.Feature{}, err
	}
	return t, nil
}

func (usecase *FeatureUseCase) UpdateFeature(ctx context.Context,
	attributes models.UpdateFeatureAttributes,
) (models.Feature, error) {
	feature, err := executor_factory.TransactionReturnValue(ctx, usecase.transactionFactory, func(
		tx repositories.Transaction,
	) (models.Feature, error) {
		feature, err := usecase.repository.GetFeatureById(ctx, tx, attributes.Id)
		if err != nil {
			return models.Feature{}, err
		}
		if err := usecase.enforceSecurity.UpdateFeature(feature); err != nil {
			return models.Feature{}, err
		}

		if err := usecase.repository.UpdateFeature(ctx, tx, attributes); err != nil {
			return models.Feature{}, err
		}
		return usecase.repository.GetFeatureById(ctx, tx, attributes.Id)
	})
	if err != nil {
		return models.Feature{}, err
	}

	tracking.TrackEvent(ctx, models.AnalyticsFeatureUpdated, map[string]interface{}{
		"feature_id": feature.Id,
	})

	return feature, err
}

func (usecase *FeatureUseCase) DeleteFeature(ctx context.Context, organizationId, featureId string) error {
	err := executor_factory.TransactionFactory.Transaction(usecase.transactionFactory, ctx, func(tx repositories.Transaction) error {
		t, err := usecase.repository.GetFeatureById(ctx, tx, featureId)
		if err != nil {
			return err
		}
		if err := usecase.enforceSecurity.DeleteFeature(t); err != nil {
			return err
		}
		if err := usecase.repository.SoftDeleteFeature(ctx, tx, featureId); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return err
	}

	tracking.TrackEvent(ctx, models.AnalyticsFeatureDeleted, map[string]interface{}{
		"feature_id": featureId,
	})

	return nil
}
