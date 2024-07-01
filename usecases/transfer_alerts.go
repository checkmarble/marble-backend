package usecases

import (
	"context"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/cockroachdb/errors"
	"github.com/guregu/null/v5"
)

type transferAlertsRepository interface {
	GetTransferAlert(ctx context.Context, exec repositories.Executor, alertId string) (models.TransferAlert, error)
	ListTransferAlerts(
		ctx context.Context,
		exec repositories.Executor,
		organizationId string,
		partnerId string,
		senderOrBeneficiary string,
		transferId null.String,
	) ([]models.TransferAlert, error)
	CreateTransferAlert(
		ctx context.Context,
		exec repositories.Executor,
		alert models.TransferAlert,
	) error
	UpdateTransferAlertAsSender(
		ctx context.Context,
		exec repositories.Executor,
		alertId string,
		input models.TransferAlertUpdateBodySender,
	) error
	UpdateTransferAlertAsBeneficiary(
		ctx context.Context,
		exec repositories.Executor,
		alertId string,
		input models.TransferAlertUpdateBodyBeneficiary,
	) error
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
		senderOrBeneficiary string,
	) error
}

type transferDataReader interface {
	QueryTransferDataFromMapping(
		ctx context.Context,
		db repositories.Executor,
		transferMapping models.TransferMapping,
	) ([]models.TransferData, error)
}

type TransferAlertsUsecase struct {
	enforceSecurity            enforceSecurityTransferAlerts
	executorFactory            executor_factory.ExecutorFactory
	organizationRepository     repositories.OrganizationRepository
	transactionFactory         executor_factory.TransactionFactory
	transferMappingsRepository transferMappingsRepository
	transferAlertsRepository   transferAlertsRepository
	partnersRepository         partnersRepository
	transferDataReader         transferDataReader
}

func NewTransferAlertsUsecase(
	enforceSecurity enforceSecurityTransferAlerts,
	executorFactory executor_factory.ExecutorFactory,
	organizationRepository repositories.OrganizationRepository,
	transactionFactory executor_factory.TransactionFactory,
	transferMappingsRepository transferMappingsRepository,
	transferAlertsRepository transferAlertsRepository,
	partnersRepository partnersRepository,
	transferDataReader transferDataReader,
) TransferAlertsUsecase {
	return TransferAlertsUsecase{
		enforceSecurity:            enforceSecurity,
		executorFactory:            executorFactory,
		organizationRepository:     organizationRepository,
		transactionFactory:         transactionFactory,
		transferMappingsRepository: transferMappingsRepository,
		transferAlertsRepository:   transferAlertsRepository,
		partnersRepository:         partnersRepository,
		transferDataReader:         transferDataReader,
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

func (usecase TransferAlertsUsecase) GetTransferAlert(ctx context.Context, alertId string, senderOrBeneficiary string) (models.TransferAlert, error) {
	exec := usecase.executorFactory.NewExecutor()
	alert, err := usecase.transferAlertsRepository.GetTransferAlert(ctx, exec, alertId)
	if err != nil {
		return models.TransferAlert{}, err
	}

	err = usecase.enforceSecurity.ReadTransferAlert(ctx, alert, senderOrBeneficiary)
	if err != nil {
		return models.TransferAlert{}, err
	}

	return alert, nil
}

func (usecase TransferAlertsUsecase) ListTransferAlerts(
	ctx context.Context,
	organizationId string,
	partnerId string,
	senderOrBeneficiary string,
	transferId null.String,
) ([]models.TransferAlert, error) {
	exec := usecase.executorFactory.NewExecutor()
	alerts, err := usecase.transferAlertsRepository.ListTransferAlerts(
		ctx,
		exec,
		organizationId,
		partnerId,
		senderOrBeneficiary,
		transferId,
	)
	if err != nil {
		return nil, err
	}

	for _, alert := range alerts {
		err = usecase.enforceSecurity.ReadTransferAlert(ctx, alert, senderOrBeneficiary)
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

	if err := input.Validate(); err != nil {
		return models.TransferAlert{}, err
	}

	_, err = usecase.validateOrgHasTransfercheckEnabled(ctx, input.OrganizationId)
	if err != nil {
		return models.TransferAlert{}, err
	}

	// -------
	// Bloc: we verify that there is a transfer with this id, that its beneficiary bank is in the network, and find the corresponding partner
	exec := usecase.executorFactory.NewExecutor()
	transferMapping, err := usecase.transferMappingsRepository.GetTransferMapping(ctx, exec, input.TransferId)
	if err != nil {
		return models.TransferAlert{}, err
	}
	if transferMapping.PartnerId != input.SenderPartnerId {
		return models.TransferAlert{}, errors.Wrapf(
			models.NotFoundError,
			"transfer %s not found for partner %s", input.TransferId, input.SenderPartnerId,
		)
	}

	// read the actual transfer data
	transfers, err := usecase.transferDataReader.QueryTransferDataFromMapping(ctx, nil, transferMapping)
	if err != nil {
		return models.TransferAlert{}, err
	}
	if len(transfers) == 0 {
		return models.TransferAlert{}, errors.Newf("no ingested object found for transferId %s", input.TransferId)
	}
	transferData := transfers[0]

	bic := transferData.BeneficiaryBic
	if bic == "" {
		return models.TransferAlert{}, errors.Wrapf(
			models.BadParameterError,
			"beneficiary_bic not found in ingested object %s", input.TransferId)
	}

	// finally, find the partner
	partnersByBic, err := usecase.partnersRepository.ListPartners(ctx, exec, models.PartnerFilters{
		Bic: null.StringFrom(bic),
	})
	if err != nil {
		return models.TransferAlert{}, err
	}
	if len(partnersByBic) == 0 {
		return models.TransferAlert{}, errors.Wrapf(models.BadParameterError, "partner not found for bic %s", bic)
	}
	// Bloc end
	// -------

	alert, err := input.WithBeneficiaryPartnerAndDefaults(partnersByBic[0].Id)
	if err != nil {
		return models.TransferAlert{}, err
	}

	return executor_factory.TransactionReturnValue(
		ctx,
		usecase.transactionFactory,
		func(tx repositories.Executor) (models.TransferAlert, error) {
			err := usecase.transferAlertsRepository.CreateTransferAlert(ctx, tx, alert)
			if err != nil {
				return models.TransferAlert{}, err
			}

			return usecase.transferAlertsRepository.GetTransferAlert(ctx, tx, alert.Id)
		},
	)
}

func (usecase TransferAlertsUsecase) UpdateTransferAlertAsSender(
	ctx context.Context,
	alertId string,
	input models.TransferAlertUpdateBodySender,
	organizationId string,
) (models.TransferAlert, error) {
	_, err := usecase.validateOrgHasTransfercheckEnabled(ctx, organizationId)
	if err != nil {
		return models.TransferAlert{}, err
	}

	if err := input.Validate(); err != nil {
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

func (usecase TransferAlertsUsecase) UpdateTransferAlertAsBeneficiary(
	ctx context.Context,
	alertId string,
	input models.TransferAlertUpdateBodyBeneficiary,
	organizationId string,
) (models.TransferAlert, error) {
	_, err := usecase.validateOrgHasTransfercheckEnabled(ctx, organizationId)
	if err != nil {
		return models.TransferAlert{}, err
	}

	if err := input.Validate(); err != nil {
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
			err = usecase.enforceSecurity.UpdateTransferAlert(ctx, alert, "beneficiary")
			if err != nil {
				return models.TransferAlert{}, err
			}

			err = usecase.transferAlertsRepository.UpdateTransferAlertAsBeneficiary(ctx, tx, alertId, input)
			if err != nil {
				return models.TransferAlert{}, err
			}

			return usecase.transferAlertsRepository.GetTransferAlert(ctx, tx, alertId)
		},
	)
}
