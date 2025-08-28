package usecases

import (
	"context"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/checkmarble/marble-backend/usecases/security"
	"github.com/cockroachdb/errors"
)

type aiSettingRepository interface {
	GetAiSetting(ctx context.Context, exec repositories.Executor, orgId string) (*models.AiSetting, error)
	UpsertAiSetting(ctx context.Context, exec repositories.Executor, orgId string, setting models.UpsertAiSetting) error
}

type organizationRepository interface {
	GetOrganizationById(ctx context.Context, exec repositories.Executor, orgId string) (models.Organization, error)
}

type AiSettingUsecase struct {
	executorFactory executor_factory.ExecutorFactory
	enforceSecurity security.EnforceSecurityOrganization
	repository      aiSettingRepository
	orgRepository   organizationRepository
}

func (uc AiSettingUsecase) GetAiSetting(ctx context.Context, orgId string) (*models.AiSetting, error) {
	if err := uc.enforceSecurity.ReadOrganization(orgId); err != nil {
		return nil, errors.Wrap(err, "don't have permission to see organization setting")
	}

	return uc.repository.GetAiSetting(ctx, uc.executorFactory.NewExecutor(), orgId)
}

func (uc AiSettingUsecase) UpsertAiSetting(ctx context.Context, orgId string,
	newSetting models.UpsertAiSetting,
) (models.AiSetting, error) {
	exec := uc.executorFactory.NewExecutor()

	org, err := uc.orgRepository.GetOrganizationById(ctx, exec, orgId)
	if err != nil {
		return models.AiSetting{}, errors.Wrap(err, "could not retrieve organization")
	}
	if err := uc.enforceSecurity.EditOrganization(org); err != nil {
		return models.AiSetting{}, errors.Wrap(err,
			"don't have permission to update organization setting")
	}

	if err := uc.repository.UpsertAiSetting(ctx, uc.executorFactory.NewExecutor(), orgId, newSetting); err != nil {
		return models.AiSetting{}, errors.Wrap(err, "can't upsert ai setting")
	}

	aiSettingUpdated, err := uc.repository.GetAiSetting(ctx, exec, orgId)
	if err != nil {
		return models.AiSetting{}, errors.Wrap(err, "can't get ai setting after update")
	}
	if aiSettingUpdated == nil {
		return models.AiSetting{}, errors.New("Ai setting is null after upsert")
	}

	return *aiSettingUpdated, nil
}
