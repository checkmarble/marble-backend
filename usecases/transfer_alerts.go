package usecases

import (
	"context"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
)

type transferAlertsRepository interface {
	GetTransferAlert(ctx context.Context, exec repositories.Executor, alertId string) (models.TransferAlert, error)
	ListTransferAlerts(
		ctx context.Context,
		exec repositories.Executor,
		partnerId string,
		senderOrReceiver string) ([]models.TransferAlert, error)
	CreateTransferAlert(ctx context.Context, exec repositories.Executor,
		alert models.TransferAlertCreateBody) error
	UpdateTransferAlertAsSender(
		ctx context.Context,
		exec repositories.Executor,
		alertId string,
		input models.TransferAlertUpdateBodySender) error
	UpdateTransferAlertAsReceiver(
		ctx context.Context,
		exec repositories.Executor,
		alertId string,
		input models.TransferAlertUpdateBodyReceiver) error
}

type enforceSecurityTransferAlerts interface {
	ReadTransferAlert(
		ctx context.Context,
		transferAlert models.TransferAlert,
		accessType string,
	) error
	CreateTransferAlert(
		ctx context.Context,
		organizationId string,
		partnerId string,
	) error
	UpdateTransferAlert(
		ctx context.Context,
		transferAlert models.TransferAlert,
		senderOrReceiver string,
	) error
}

type TransferAlertsUsecase struct {
	enforceSecurity            enforceSecurityTransferAlerts
	executorFactory            executor_factory.ExecutorFactory
	organizationRepository     repositories.OrganizationRepository
	transactionFactory         executor_factory.TransactionFactory
	transferMappingsRepository transferMappingsRepository
	transferAlertsRepository   transferAlertsRepository
}

func NewTransferAlertsUsecase(
	enforceSecurity enforceSecurityTransferAlerts,
	executorFactory executor_factory.ExecutorFactory,
	organizationRepository repositories.OrganizationRepository,
	transactionFactory executor_factory.TransactionFactory,
	transferMappingsRepository transferMappingsRepository,
	transferAlertsRepository transferAlertsRepository,
) TransferAlertsUsecase {
	return TransferAlertsUsecase{
		enforceSecurity:            enforceSecurity,
		executorFactory:            executorFactory,
		organizationRepository:     organizationRepository,
		transactionFactory:         transactionFactory,
		transferMappingsRepository: transferMappingsRepository,
		transferAlertsRepository:   transferAlertsRepository,
	}
}

func (usecase TransferAlertsUsecase) validateOrgHasTransfercheckEnabled(ctx context.Context, organizationId string) (scenarioId string, err error) {
	org, err := usecase.organizationRepository.GetOrganizationById(
		ctx,
		usecase.executorFactory.NewExecutor(),
		organizationId,
	)
	if err != nil {
		return "", err
	}
	if org.TransferCheckScenarioId == nil {
		return "", errors.Wrapf(models.ForbiddenError,
			"organization %s is not setup for transfer check", organizationId,
		)
	}
	return *org.TransferCheckScenarioId, nil
}

func (usecase TransferAlertsUsecase) GetTransferAlert(ctx context.Context, alertId string, senderOrReceiver string) (models.TransferAlert, error) {
	_, err := usecase.validateOrgHasTransfercheckEnabled(ctx, alertId)
	if err != nil {
		return models.TransferAlert{}, err
	}

	exec := usecase.executorFactory.NewExecutor()
	alert, err := usecase.transferAlertsRepository.GetTransferAlert(ctx, exec, alertId)
	if err != nil {
		return models.TransferAlert{}, err
	}

	err = usecase.enforceSecurity.ReadTransferAlert(ctx, alert, senderOrReceiver)
	if err != nil {
		return models.TransferAlert{}, err
	}

	return alert, nil
}

func (usecase TransferAlertsUsecase) ListTransferAlerts(ctx context.Context, partnerId *string, senderOrReceiver string) ([]models.TransferAlert, error) {
	if partnerId == nil {
		return nil, errors.Wrap(models.ForbiddenError, "partner id is required")
	}
	exec := usecase.executorFactory.NewExecutor()
	alerts, err := usecase.transferAlertsRepository.ListTransferAlerts(ctx, exec, *partnerId, senderOrReceiver)
	if err != nil {
		return nil, err
	}

	for _, alert := range alerts {
		err = usecase.enforceSecurity.ReadTransferAlert(ctx, alert, senderOrReceiver)
		if err != nil {
			return nil, err
		}
	}

	return alerts, nil
}

func (usecase TransferAlertsUsecase) CreateTransferAlert(
	ctx context.Context,
	input models.TransferAlertCreateBody,
) (models.TransferAlert, error) {
	err := usecase.enforceSecurity.CreateTransferAlert(ctx, input.OrganizationId, input.SenderPartnerId)
	if err != nil {
		return models.TransferAlert{}, err
	}

	_, err = usecase.validateOrgHasTransfercheckEnabled(ctx, input.OrganizationId)
	if err != nil {
		return models.TransferAlert{}, err
	}

	exec := usecase.executorFactory.NewExecutor()
	_, err = usecase.transferMappingsRepository.GetTransferMapping(ctx, exec, input.TransferId)
	if err != nil {
		return models.TransferAlert{}, err
	}
	// TODO: get beneficiary partner id, need to merge another PR first
	input.BeneficiaryPartnerId = uuid.NewString()

	input.Id = uuid.NewString()
	return executor_factory.TransactionReturnValue(
		ctx,
		usecase.transactionFactory,
		func(tx repositories.Executor) (models.TransferAlert, error) {
			err := usecase.transferAlertsRepository.CreateTransferAlert(ctx, tx, input)
			if err != nil {
				return models.TransferAlert{}, err
			}

			return usecase.transferAlertsRepository.GetTransferAlert(ctx, tx, input.Id)
		},
	)
}

func (usecase TransferAlertsUsecase) UpcateTransferAlertAsSender(
	ctx context.Context,
	alertId string,
	input models.TransferAlertUpdateBodySender,
	organizationId string,
) (models.TransferAlert, error) {
	_, err := usecase.validateOrgHasTransfercheckEnabled(ctx, organizationId)
	if err != nil {
		return models.TransferAlert{}, err
	}

	return executor_factory.TransactionReturnValue(
		ctx,
		usecase.transactionFactory,
		func(tx repositories.Executor) (models.TransferAlert, error) {
			alert, err := usecase.transferAlertsRepository.GetTransferAlert(ctx, tx, alertId)
			if err != nil {
				return models.TransferAlert{}, err
			}
			err = usecase.enforceSecurity.UpdateTransferAlert(ctx, alert, "sender")
			if err != nil {
				return models.TransferAlert{}, err
			}

			err = usecase.transferAlertsRepository.UpdateTransferAlertAsSender(ctx, tx, alertId, input)
			if err != nil {
				return models.TransferAlert{}, err
			}

			return usecase.transferAlertsRepository.GetTransferAlert(ctx, tx, alertId)
		},
	)
}

func (usecase TransferAlertsUsecase) UpcateTransferAlertAsReceiver(
	ctx context.Context,
	alertId string,
	input models.TransferAlertUpdateBodyReceiver,
	organizationId string,
) (models.TransferAlert, error) {
	_, err := usecase.validateOrgHasTransfercheckEnabled(ctx, organizationId)
	if err != nil {
		return models.TransferAlert{}, err
	}

	return executor_factory.TransactionReturnValue(
		ctx,
		usecase.transactionFactory,
		func(tx repositories.Executor) (models.TransferAlert, error) {
			alert, err := usecase.transferAlertsRepository.GetTransferAlert(ctx, tx, alertId)
			if err != nil {
				return models.TransferAlert{}, err
			}
			err = usecase.enforceSecurity.UpdateTransferAlert(ctx, alert, "receiver")
			if err != nil {
				return models.TransferAlert{}, err
			}

			err = usecase.transferAlertsRepository.UpdateTransferAlertAsReceiver(ctx, tx, alertId, input)
			if err != nil {
				return models.TransferAlert{}, err
			}

			return usecase.transferAlertsRepository.GetTransferAlert(ctx, tx, alertId)
		},
	)
}
