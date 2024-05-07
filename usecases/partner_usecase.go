package usecases

import (
	"context"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/google/uuid"
)

type partnersRepository interface {
	ListPartners(ctx context.Context, exec repositories.Executor) ([]models.Partner, error)
	CreatePartner(
		ctx context.Context,
		exec repositories.Executor,
		partnerId string,
		partnerCreateInput models.PartnerCreateInput) error
	UpdatePartner(
		ctx context.Context,
		exec repositories.Executor,
		partnerId string,
		partnerUpdateInput models.PartnerUpdateInput) error
	GetPartnerById(ctx context.Context, exec repositories.Executor, partnerId string) (models.Partner, error)
}

type enforceSecurityPartner interface {
	ListPartners(ctx context.Context) error
	CreatePartner(ctx context.Context) error
	ReadPartner(ctx context.Context, partnerId string) error
	UpdatePartner(ctx context.Context) error
}

type PartnerUsecase struct {
	enforceSecurity    enforceSecurityPartner
	transactionFactory executor_factory.TransactionFactory
	executorFactory    executor_factory.ExecutorFactory
	partnersRepository partnersRepository
}

func (usecase *PartnerUsecase) ListPartners(ctx context.Context) ([]models.Partner, error) {
	if err := usecase.enforceSecurity.ListPartners(ctx); err != nil {
		return nil, err
	}

	exec := usecase.executorFactory.NewExecutor()
	partners, err := usecase.partnersRepository.ListPartners(ctx, exec)
	if err != nil {
		return []models.Partner{}, err
	}
	return partners, nil
}

func (usecase *PartnerUsecase) CreatePartner(
	ctx context.Context,
	partnerCreateInput models.PartnerCreateInput,
) (models.Partner, error) {
	if err := usecase.enforceSecurity.CreatePartner(ctx); err != nil {
		return models.Partner{}, err
	}

	partner, err := executor_factory.TransactionReturnValue(ctx, usecase.transactionFactory, func(
		exec repositories.Executor,
	) (models.Partner, error) {
		partnerId := uuid.New().String()
		if err := usecase.partnersRepository.CreatePartner(ctx, exec, partnerId, partnerCreateInput); err != nil {
			return models.Partner{}, err
		}
		return usecase.partnersRepository.GetPartnerById(ctx, exec, partnerId)
	})
	if err != nil {
		return models.Partner{}, err
	}
	return partner, nil
}

func (usecase *PartnerUsecase) GetPartner(
	ctx context.Context,
	partnerId string,
) (models.Partner, error) {
	exec := usecase.executorFactory.NewExecutor()
	partner, err := usecase.partnersRepository.GetPartnerById(ctx, exec, partnerId)
	if err != nil {
		return models.Partner{}, err
	}

	if err := usecase.enforceSecurity.ReadPartner(ctx, partner.Id); err != nil {
		return models.Partner{}, err
	}

	return partner, nil
}

func (usecase *PartnerUsecase) UpdatePartner(
	ctx context.Context,
	partnerId string,
	partnerUpdateInput models.PartnerUpdateInput,
) (models.Partner, error) {
	if err := usecase.enforceSecurity.UpdatePartner(ctx); err != nil {
		return models.Partner{}, err
	}

	partner, err := executor_factory.TransactionReturnValue(ctx, usecase.transactionFactory, func(
		exec repositories.Executor,
	) (models.Partner, error) {
		if err := usecase.partnersRepository.UpdatePartner(ctx, exec, partnerId, partnerUpdateInput); err != nil {
			return models.Partner{}, err
		}
		return usecase.partnersRepository.GetPartnerById(ctx, exec, partnerId)
	})
	if err != nil {
		return models.Partner{}, err
	}
	return partner, nil
}
