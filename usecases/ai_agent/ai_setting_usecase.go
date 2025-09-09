package ai_agent

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
	PutAiSetting(
		ctx context.Context,
		exec repositories.Executor,
		orgId string,
		setting models.UpsertAiSetting,
	) (models.AiSetting, error)
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

func NewAiSettingUsecase(
	executorFactory executor_factory.ExecutorFactory,
	enforceSecurity security.EnforceSecurityOrganization,
	repository aiSettingRepository,
	orgRepository organizationRepository,
) AiSettingUsecase {
	return AiSettingUsecase{
		executorFactory: executorFactory,
		enforceSecurity: enforceSecurity,
		repository:      repository,
		orgRepository:   orgRepository,
	}
}

func (uc AiSettingUsecase) GetAiSetting(ctx context.Context, orgId string) (models.AiSetting, error) {
	if err := uc.enforceSecurity.ReadOrganization(orgId); err != nil {
		return models.AiSetting{}, errors.Wrap(err,
			"don't have permission to see organization setting")
	}

	aiSetting, err := uc.repository.GetAiSetting(ctx, uc.executorFactory.NewExecutor(), orgId)
	if err != nil {
		return models.AiSetting{}, errors.Wrap(err, "could not retrieve ai setting")
	}

	if aiSetting == nil {
		return models.DefaultAiSetting(), nil
	}

	return *aiSetting, nil
}

func (uc AiSettingUsecase) PutAiSetting(
	ctx context.Context,
	orgId string,
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

	aiSettingPatched, err := uc.repository.PutAiSetting(ctx, exec, orgId, newSetting)
	if err != nil {
		return models.AiSetting{}, errors.Wrap(err, "can't upsert ai setting")
	}

	return aiSettingPatched, nil
}
