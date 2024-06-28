package usecases

import (
	"context"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
	"github.com/guregu/null/v5"
)

type transferAlertsRepository interface {
	GetTransferAlert(ctx context.Context, exec repositories.Executor, alertId string) (models.TransferAlert, error)
	ListTransferAlerts(
		ctx context.Context,
		exec repositories.Executor,
		organizationId string,
		partnerId string,
		senderOrReceiver string,
	) ([]models.TransferAlert, error)
	CreateTransferAlert(
		ctx context.Context,
		exec repositories.Executor,
		alert models.TransferAlertCreateBody,
	) error
	UpdateTransferAlertAsSender(
		ctx context.Context,
		exec repositories.Executor,
		alertId string,
		input models.TransferAlertUpdateBodySender,
	) error
	UpdateTransferAlertAsReceiver(
		ctx context.Context,
		exec repositories.Executor,
		alertId string,
		input models.TransferAlertUpdateBodyReceiver,
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
	partnersRepository         partnersRepository
	ingestedDataReadRepository repositories.IngestedDataReadRepository
	dataModelRepository        repositories.DataModelRepository
}

func NewTransferAlertsUsecase(
	enforceSecurity enforceSecurityTransferAlerts,
	executorFactory executor_factory.ExecutorFactory,
	organizationRepository repositories.OrganizationRepository,
	transactionFactory executor_factory.TransactionFactory,
	transferMappingsRepository transferMappingsRepository,
	transferAlertsRepository transferAlertsRepository,
	partnersRepository partnersRepository,
	ingestedDataReadRepository repositories.IngestedDataReadRepository,
	dataModelRepository repositories.DataModelRepository,
) TransferAlertsUsecase {
	return TransferAlertsUsecase{
		enforceSecurity:            enforceSecurity,
		executorFactory:            executorFactory,
		organizationRepository:     organizationRepository,
		transactionFactory:         transactionFactory,
		transferMappingsRepository: transferMappingsRepository,
		transferAlertsRepository:   transferAlertsRepository,
		partnersRepository:         partnersRepository,
		ingestedDataReadRepository: ingestedDataReadRepository,
		dataModelRepository:        dataModelRepository,
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

func (usecase TransferAlertsUsecase) ListTransferAlerts(
	ctx context.Context,
	organizationId string,
	partnerId string,
	senderOrReceiver string,
) ([]models.TransferAlert, error) {
	exec := usecase.executorFactory.NewExecutor()
	alerts, err := usecase.transferAlertsRepository.ListTransferAlerts(
		ctx,
		exec,
		organizationId,
		partnerId,
		senderOrReceiver,
	)
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

	// -------
	// Bloc: we verify that there is a transfer with this id and that it's beneficiary bank is in the network
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
	// TODO: factorize some of this, it's used both in here and in the transfer check usecase
	db, err := usecase.executorFactory.NewClientDbExecutor(ctx, input.OrganizationId)
	if err != nil {
		return models.TransferAlert{}, err
	}

	dataModel, err := usecase.dataModelRepository.GetDataModel(ctx, exec, input.OrganizationId, false)
	if err != nil {
		return models.TransferAlert{}, err
	}
	table, ok := dataModel.Tables[TransferCheckTable]
	if !ok {
		return models.TransferAlert{}, errors.Newf("table %s not found", TransferCheckTable)
	}

	objectId := models.ObjectIdWithPartnerIdPrefix(transferMapping.PartnerId, transferMapping.ClientTransferId)
	objects, err := usecase.ingestedDataReadRepository.QueryIngestedObject(ctx, db, table, objectId)
	if err != nil {
		return models.TransferAlert{}, errors.Wrap(err,
			"error while querying ingested objects in lookupPreviousObjects")
	}
	if len(objects) == 0 {
		return models.TransferAlert{}, errors.Newf("no ingested object found for transferId %s", input.TransferId)
	}

	bic, ok := objects[0]["bic"].(string)
	if !ok {
		return models.TransferAlert{}, errors.New("bic not found in ingested object")
	}

	partnersByBic, err := usecase.partnersRepository.ListPartners(ctx, exec, models.PartnerFilters{
		Bic: null.StringFrom(bic),
	})
	if err != nil {
		return models.TransferAlert{}, err
	}
	if len(partnersByBic) == 0 {
		return models.TransferAlert{}, errors.Wrapf(models.BadParameterError, "partner not found for bic %s", bic)
	}
	input.BeneficiaryPartnerId = partnersByBic[0].Id
	// Bloc end
	// -------

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

func (usecase TransferAlertsUsecase) UpdateTransferAlertAsReceiver(
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
