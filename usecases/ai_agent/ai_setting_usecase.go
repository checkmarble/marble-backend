package ai_agent

import (
	"context"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/checkmarble/marble-backend/usecases/security"
	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
)

type aiSettingRepository interface {
	GetAiSettingById(ctx context.Context, exec repositories.Executor, id uuid.UUID) (*models.AiSetting, error)
	UpsertAiSetting(
		ctx context.Context,
		exec repositories.Executor,
		aiSettingId *uuid.UUID,
		setting models.UpsertAiSetting,
	) (models.AiSetting, error)
}

type organizationRepository interface {
	GetOrganizationById(ctx context.Context, exec repositories.Executor, orgId string) (models.Organization, error)
	UpdateOrganizationAiSettingId(
		ctx context.Context,
		exec repositories.Executor,
		organizationId string,
		aiSettingId *uuid.UUID,
	) error
}

type AiSettingUsecase struct {
	executorFactory    executor_factory.ExecutorFactory
	transactionFactory executor_factory.TransactionFactory
	enforceSecurity    security.EnforceSecurityOrganization
	repository         aiSettingRepository
	orgRepository      organizationRepository
}

func NewAiSettingUsecase(
	executorFactory executor_factory.ExecutorFactory,
	transactionFactory executor_factory.TransactionFactory,
	enforceSecurity security.EnforceSecurityOrganization,
	repository aiSettingRepository,
	orgRepository organizationRepository,
) AiSettingUsecase {
	return AiSettingUsecase{
		executorFactory:    executorFactory,
		transactionFactory: transactionFactory,
		enforceSecurity:    enforceSecurity,
		repository:         repository,
		orgRepository:      orgRepository,
	}
}

func (uc AiSettingUsecase) GetAiSetting(ctx context.Context, orgId string) (*models.AiSetting, error) {
	if err := uc.enforceSecurity.ReadOrganization(orgId); err != nil {
		return nil, errors.Wrap(err, "don't have permission to see organization setting")
	}

	org, err := uc.orgRepository.GetOrganizationById(ctx, uc.executorFactory.NewExecutor(), orgId)
	if err != nil {
		return nil, errors.Wrap(err, "could not retrieve organization")
	}

	if org.AiSettingId == nil {
		return nil, nil
	}

	return uc.repository.GetAiSettingById(ctx, uc.executorFactory.NewExecutor(), *org.AiSettingId)
}

func (uc AiSettingUsecase) UpsertAiSetting(
	ctx context.Context,
	orgId string,
	newSetting models.UpsertAiSetting,
) (models.AiSetting, error) {
	return executor_factory.TransactionReturnValue(
		ctx,
		uc.transactionFactory,
		func(tx repositories.Transaction) (models.AiSetting, error) {
			org, err := uc.orgRepository.GetOrganizationById(ctx, tx, orgId)
			if err != nil {
				return models.AiSetting{}, errors.Wrap(err, "could not retrieve organization")
			}
			if err := uc.enforceSecurity.EditOrganization(org); err != nil {
				return models.AiSetting{}, errors.Wrap(err,
					"don't have permission to update organization setting")
			}

			aiSettingUpserted, err := uc.repository.UpsertAiSetting(ctx, tx, org.AiSettingId, newSetting)
			if err != nil {
				return models.AiSetting{}, errors.Wrap(err, "can't upsert ai setting")
			}

			err = uc.orgRepository.UpdateOrganizationAiSettingId(ctx, tx, orgId, &aiSettingUpserted.Id)
			if err != nil {
				return models.AiSetting{}, errors.Wrap(err,
					"can't update organization to attach ai setting",
				)
			}

			return aiSettingUpserted, nil
		},
	)
}
