package ai_agent

import (
	"context"

	"github.com/checkmarble/marble-backend/models"
	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
)

func (uc *AiAgentUsecase) GetAiSetting(ctx context.Context, orgId uuid.UUID) (models.AiSetting, error) {
	if err := uc.enforceSecurityOrganization.ReadOrganization(orgId); err != nil {
		return models.AiSetting{}, errors.Wrap(err,
			"don't have permission to see organization setting")
	}

	aiSetting, err := uc.getAiSetting(ctx, orgId)
	if err != nil {
		return models.AiSetting{}, errors.Wrap(err, "could not get ai setting")
	}

	return aiSetting, nil
}

func (uc *AiAgentUsecase) PutAiSetting(
	ctx context.Context,
	orgId uuid.UUID,
	newSetting models.UpsertAiSetting,
) (models.AiSetting, error) {
	exec := uc.executorFactory.NewExecutor()
	org, err := uc.repository.GetOrganizationById(ctx, exec, orgId)
	if err != nil {
		return models.AiSetting{}, errors.Wrap(err, "could not retrieve organization")
	}
	if err := uc.enforceSecurityOrganization.EditOrganization(org); err != nil {
		return models.AiSetting{}, errors.Wrap(err,
			"don't have permission to update organization setting")
	}

	aiSettingPatched, err := uc.repository.PutAiSetting(ctx, exec, orgId, newSetting)
	if err != nil {
		return models.AiSetting{}, errors.Wrap(err, "can't upsert ai setting")
	}

	return aiSettingPatched, nil
}
