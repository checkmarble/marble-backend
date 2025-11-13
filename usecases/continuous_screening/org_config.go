package continuous_screening

import (
	"context"

	"github.com/checkmarble/marble-backend/models"
	"github.com/google/uuid"
)

func (uc *ContinuousScreeningUsecase) GetContinuousScreeningConfig(ctx context.Context, id uuid.UUID) (models.ContinuousScreeningConfig, error) {
	config, err := uc.repository.GetContinuousScreeningConfig(ctx, uc.executorFactory.NewExecutor(), id)
	if err != nil {
		return models.ContinuousScreeningConfig{}, err
	}

	if err := uc.enforceSecurity.ReadContinuousScreeningConfig(config); err != nil {
		return models.ContinuousScreeningConfig{}, err
	}

	return config, nil
}

func (uc *ContinuousScreeningUsecase) GetContinuousScreeningConfigsByOrgId(ctx context.Context, orgId string) ([]models.ContinuousScreeningConfig, error) {
	configs, err := uc.repository.GetContinuousScreeningConfigsByOrgId(ctx,
		uc.executorFactory.NewExecutor(), orgId)
	if err != nil {
		return []models.ContinuousScreeningConfig{}, err
	}

	for _, config := range configs {
		if err := uc.enforceSecurity.ReadContinuousScreeningConfig(config); err != nil {
			return []models.ContinuousScreeningConfig{}, err
		}
	}

	return configs, nil
}

func (uc *ContinuousScreeningUsecase) CreateContinuousScreeningConfig(
	ctx context.Context,
	input models.CreateContinuousScreeningConfig,
) (models.ContinuousScreeningConfig, error) {
	if err := uc.enforceSecurity.WriteContinuousScreeningConfig(input.OrgId); err != nil {
		return models.ContinuousScreeningConfig{}, err
	}

	configCreated, err := uc.repository.CreateContinuousScreeningConfig(ctx,
		uc.executorFactory.NewExecutor(), input)
	if err != nil {
		return models.ContinuousScreeningConfig{}, err
	}

	return configCreated, nil
}

func (uc *ContinuousScreeningUsecase) UpdateContinuousScreeningConfig(
	ctx context.Context,
	id uuid.UUID,
	input models.UpdateContinuousScreeningConfig,
) (models.ContinuousScreeningConfig, error) {
	exec := uc.executorFactory.NewExecutor()
	config, err := uc.repository.GetContinuousScreeningConfig(ctx, exec, id)
	if err != nil {
		return models.ContinuousScreeningConfig{}, err
	}

	if err := uc.enforceSecurity.WriteContinuousScreeningConfig(config.OrgId); err != nil {
		return models.ContinuousScreeningConfig{}, err
	}

	configUpdated, err := uc.repository.UpdateContinuousScreeningConfig(ctx, exec, id, input)
	if err != nil {
		return models.ContinuousScreeningConfig{}, err
	}

	return configUpdated, nil
}
