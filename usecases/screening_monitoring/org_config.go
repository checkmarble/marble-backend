package screening_monitoring

import (
	"context"

	"github.com/checkmarble/marble-backend/models"
	"github.com/google/uuid"
)

func (uc *ScreeningMonitoringUsecase) GetScreeningMonitoringConfig(ctx context.Context, id uuid.UUID) (models.ScreeningMonitoringConfig, error) {
	config, err := uc.screeningMonitoringRepository.GetScreeningMonitoringConfig(ctx, uc.executorFactory.NewExecutor(), id)
	if err != nil {
		return models.ScreeningMonitoringConfig{}, err
	}

	if err := uc.enforceSecurity.ReadScreeningMonitoringConfig(config); err != nil {
		return models.ScreeningMonitoringConfig{}, err
	}

	return config, nil
}

func (uc *ScreeningMonitoringUsecase) GetScreeningMonitoringConfigsByOrgId(ctx context.Context, orgId string) ([]models.ScreeningMonitoringConfig, error) {
	configs, err := uc.screeningMonitoringRepository.GetScreeningMonitoringConfigsByOrgId(ctx,
		uc.executorFactory.NewExecutor(), orgId)
	if err != nil {
		return []models.ScreeningMonitoringConfig{}, err
	}

	for _, config := range configs {
		if err := uc.enforceSecurity.ReadScreeningMonitoringConfig(config); err != nil {
			return []models.ScreeningMonitoringConfig{}, err
		}
	}

	return configs, nil
}

func (uc *ScreeningMonitoringUsecase) CreateScreeningMonitoringConfig(
	ctx context.Context,
	input models.CreateScreeningMonitoringConfig,
) (models.ScreeningMonitoringConfig, error) {
	if err := uc.enforceSecurity.WriteScreeningMonitoringConfig(input.OrgId); err != nil {
		return models.ScreeningMonitoringConfig{}, err
	}

	configCreated, err := uc.screeningMonitoringRepository.CreateScreeningMonitoringConfig(ctx,
		uc.executorFactory.NewExecutor(), input)
	if err != nil {
		return models.ScreeningMonitoringConfig{}, err
	}

	return configCreated, nil
}

func (uc *ScreeningMonitoringUsecase) UpdateScreeningMonitoringConfig(
	ctx context.Context,
	id uuid.UUID,
	input models.UpdateScreeningMonitoringConfig,
) (models.ScreeningMonitoringConfig, error) {
	exec := uc.executorFactory.NewExecutor()
	config, err := uc.screeningMonitoringRepository.GetScreeningMonitoringConfig(ctx, exec, id)
	if err != nil {
		return models.ScreeningMonitoringConfig{}, err
	}

	if err := uc.enforceSecurity.WriteScreeningMonitoringConfig(config.OrgId); err != nil {
		return models.ScreeningMonitoringConfig{}, err
	}

	configUpdated, err := uc.screeningMonitoringRepository.UpdateScreeningMonitoringConfig(ctx, exec, id, input)
	if err != nil {
		return models.ScreeningMonitoringConfig{}, err
	}

	return configUpdated, nil
}
