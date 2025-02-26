package usecases

import (
	"context"
	"fmt"
	"net/netip"
	"slices"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
	"github.com/guregu/null/v5"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/checkmarble/marble-backend/utils"
)

type transferMappingsRepository interface {
	GetTransferMapping(ctx context.Context, exec repositories.Executor, id string) (models.TransferMapping, error)
	ListTransferMappings(
		ctx context.Context,
		exec repositories.Executor,
		organizationId string,
		partnerId string,
		clientTransferId string,
	) ([]models.TransferMapping, error)
	CreateTransferMapping(
		ctx context.Context,
		exec repositories.Executor,
		id string,
		transferMapping models.TransferMappingCreateInput,
	) error
	DeleteTransferMapping(ctx context.Context, exec repositories.Executor, id string) error
}

type transferCheckEnrichmentRepository interface {
	GetIPCountry(ctx context.Context, ip netip.Addr) (string, error)
	GetIPType(ctx context.Context, ip netip.Addr) (string, error)
	GetSenderBicRiskLevel(ctx context.Context, bic string) (string, error)
}

type enforceSecurityTransferCheck interface {
	CreateTransfer(ctx context.Context, organizationId string, partnerId string) error
	ReadTransfer(ctx context.Context, transferMapping models.TransferMapping) error
	UpdateTransfer(ctx context.Context, transferMapping models.TransferMapping) error
	ReadTransferData(ctx context.Context, partnerId string) error
}

type TransferCheckUsecase struct {
	dataModelRepository               repositories.DataModelRepository
	decisionUseCase                   DecisionUsecase
	decisionRepository                repositories.DecisionRepository
	enforceSecurity                   enforceSecurityTransferCheck
	executorFactory                   executor_factory.ExecutorFactory
	ingestionRepository               repositories.IngestionRepository
	organizationRepository            repositories.OrganizationRepository
	transactionFactory                executor_factory.TransactionFactory
	transferMappingsRepository        transferMappingsRepository
	transferCheckEnrichmentRepository transferCheckEnrichmentRepository
	transferDataReader                transferDataReader
	partnersRepository                partnersRepository
}

func transfersAreDifferent(t1, t2 map[string]any) bool {
	keys := []string{"beneficiary_bic", "beneficiary_iban", "sender_account_id", "sender_bic"}
	for _, key := range keys {
		if t1[key] != t2[key] {
			return true
		}
	}
	return false
}

func (usecase *TransferCheckUsecase) CreateTransfer(
	ctx context.Context,
	organizationId string,
	partnerId *string,
	transfer models.TransferCreateBody,
) (models.Transfer, error) {
	logger := utils.LoggerFromContext(ctx)
	exec := usecase.executorFactory.NewExecutor()

	scenarioId, err := usecase.validateOrgHasTransfercheckEnabled(ctx, organizationId)
	if err != nil {
		return models.Transfer{}, err
	}

	if partnerId == nil {
		return models.Transfer{}, errors.Wrap(models.BadParameterError, "partnerId is required")
	}

	createBody, err := transfer.TransferData.FormatAndValidate()
	if err != nil {
		return models.Transfer{}, err
	}
	createBody, err = usecase.enrichTransfer(ctx, createBody)
	if err != nil {
		return models.Transfer{}, err
	}

	if err := usecase.enforceSecurity.CreateTransfer(ctx, organizationId, *partnerId); err != nil {
		return models.Transfer{}, err
	}

	table, err := usecase.getTransfercheckTable(ctx, organizationId)
	if err != nil {
		return models.Transfer{}, err
	}

	beneficiaryInNetwork, err := usecase.beneficiaryIsInNetwork(ctx, createBody)
	if err != nil {
		return models.Transfer{}, err
	}

	var transferMappingId string
	transferMappings, err := usecase.transferMappingsRepository.ListTransferMappings(
		ctx,
		exec,
		organizationId,
		*partnerId,
		createBody.TransferId,
	)
	if err != nil {
		return models.Transfer{}, err
	}
	if len(transferMappings) > 0 {
		transferMappingId = transferMappings[0].Id
	} else {
		transferMappingId = uuid.New().String()
		err = usecase.transferMappingsRepository.CreateTransferMapping(ctx, exec,
			transferMappingId, models.TransferMappingCreateInput{
				ClientTransferId: createBody.TransferId,
				OrganizationId:   organizationId,
				PartnerId:        *partnerId,
			})
		if err != nil {
			return models.Transfer{}, err
		}
		transferMapping, err := usecase.transferMappingsRepository.GetTransferMapping(ctx, exec, transferMappingId)
		if err != nil {
			return models.Transfer{}, err
		}
		transferMappings = append(transferMappings, transferMapping)
	}

	transfersData, err := usecase.transferDataReader.QueryTransferDataFromMapping(ctx, nil, transferMappings[0])
	if err != nil {
		return models.Transfer{}, err
	}

	if len(transfersData) > 0 && transfersAreDifferent(
		createBody.ToIngestionMap(transferMappings[0]),
		transfersData[0].ToIngestionMap(transferMappings[0]),
	) {
		return models.Transfer{}, errors.Wrap(
			models.ConflictError,
			fmt.Sprintf("transfer %s already exists", createBody.TransferId),
		)
	}

	clientObject := models.ClientObject{
		Data:      createBody.ToIngestionMap(transferMappings[0]),
		TableName: models.TransferCheckTable,
	}

	err = usecase.transactionFactory.TransactionInOrgSchema(
		ctx,
		organizationId,
		func(tx repositories.Transaction) error {
			_, err := usecase.ingestionRepository.IngestObjects(ctx, tx, []models.ClientObject{
				clientObject,
			}, table)
			if err != nil {
				return err
			}

			transfersData, err = usecase.transferDataReader.QueryTransferDataFromMapping(ctx, tx, transferMappings[0])
			if err != nil {
				return err
			}
			if len(transfersData) == 0 {
				return errors.Newf("no ingested object found for transferId %s", createBody.TransferId)
			}
			return nil
		},
	)
	if err != nil {
		if delErr := usecase.transferMappingsRepository.DeleteTransferMapping(ctx, exec, transferMappingId); delErr != nil {
			logger.ErrorContext(ctx, fmt.Sprintf("error while deleting transfer mapping: %s", delErr.Error()))
		}
		return models.Transfer{}, err
	}

	doScore := transfer.SkipScore == nil || !*transfer.SkipScore
	if !doScore {
		return models.Transfer{
			Id:           transferMappingId,
			TransferData: transfersData[0],
		}, nil
	}

	_, decision, err := usecase.decisionUseCase.CreateDecision(
		ctx,
		models.CreateDecisionInput{
			ScenarioId:         scenarioId,
			ClientObject:       &clientObject,
			OrganizationId:     organizationId,
			TriggerObjectTable: models.TransferCheckTable,
		},
		models.CreateDecisionParams{
			WithDecisionWebhooks:        false,
			WithRuleExecutionDetails:    false,
			WithScenarioPermissionCheck: false,
		},
	)
	if err != nil {
		return models.Transfer{}, handleTransferCheckDecisionError(err)
	}

	return models.Transfer{
		Id:                   transferMappingId,
		TransferData:         transfersData[0],
		LastScoredAt:         null.TimeFrom(decision.CreatedAt),
		Score:                scoreFromDecision(decision.Score),
		BeneficiaryInNetwork: beneficiaryInNetwork,
	}, nil
}

func (usecase *TransferCheckUsecase) enrichTransfer(
	ctx context.Context,
	transfer models.TransferData,
) (models.TransferData, error) {
	if !transfer.SenderIP.IsUnspecified() {
		country, err := usecase.transferCheckEnrichmentRepository.GetIPCountry(ctx, transfer.SenderIP)
		if err != nil {
			return models.TransferData{}, err
		}
		transfer.SenderIPCountry = country

		ipType, err := usecase.transferCheckEnrichmentRepository.GetIPType(ctx, transfer.SenderIP)
		if err != nil {
			return models.TransferData{}, err
		}
		transfer.SenderIPType = ipType
	}
	if transfer.SenderBic != "" {
		riskLevel, err := usecase.transferCheckEnrichmentRepository.GetSenderBicRiskLevel(ctx, transfer.SenderBic)
		if err != nil {
			return models.TransferData{}, err
		}
		transfer.SenderBicRiskLevel = riskLevel
	}
	return transfer, nil
}

func (usecase *TransferCheckUsecase) UpdateTransfer(
	ctx context.Context,
	organizationId string,
	id string,
	transfer models.TransferUpdateBody,
) (models.Transfer, error) {
	exec := usecase.executorFactory.NewExecutor()

	err := validateTranferUpdate(transfer)
	if err != nil {
		return models.Transfer{}, err
	}

	transferMapping, err := usecase.transferMappingsRepository.GetTransferMapping(ctx, exec, id)
	if err != nil {
		return models.Transfer{}, err
	}

	if err := usecase.enforceSecurity.UpdateTransfer(ctx, transferMapping); err != nil {
		return models.Transfer{}, err
	}

	objectId := models.ObjectIdWithPartnerIdPrefix(transferMapping.PartnerId, transferMapping.ClientTransferId)
	previousDecisions, err := usecase.decisionRepository.DecisionsByObjectId(ctx, exec, organizationId, objectId)
	if err != nil {
		return models.Transfer{}, err
	}

	table, err := usecase.getTransfercheckTable(ctx, organizationId)
	if err != nil {
		return models.Transfer{}, err
	}

	var beneficiaryInNetwork bool
	var transfersData []models.TransferData
	err = usecase.transactionFactory.TransactionInOrgSchema(ctx, organizationId, func(tx repositories.Transaction) error {
		transfersData, err = usecase.transferDataReader.QueryTransferDataFromMapping(ctx, tx, transferMapping)
		if err != nil {
			return err
		}
		if len(transfersData) == 0 {
			return errors.Wrap(
				models.NotFoundError,
				fmt.Sprintf("transfer %s not found", transferMapping.ClientTransferId),
			)
		}

		beneficiaryInNetwork, err = usecase.beneficiaryIsInNetwork(ctx, transfersData[0])
		if err != nil {
			return err
		}

		previous := transfersData[0].ToIngestionMap(transferMapping)
		previous["status"] = transfer.Status
		previous["updated_at"] = time.Now()

		_, err = usecase.ingestionRepository.IngestObjects(ctx, tx, []models.ClientObject{
			{Data: previous, TableName: models.TransferCheckTable},
		}, table)
		if err != nil {
			return err
		}

		transfersData, err = usecase.transferDataReader.QueryTransferDataFromMapping(ctx, tx, transferMapping)
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return models.Transfer{}, err
	}

	out := models.Transfer{
		Id:                   id,
		TransferData:         transfersData[0],
		BeneficiaryInNetwork: beneficiaryInNetwork,
	}
	if len(previousDecisions) > 0 {
		out.LastScoredAt = null.TimeFrom(previousDecisions[0].CreatedAt)
		out.Score = scoreFromDecision(previousDecisions[0].Score)
	}

	return out, nil
}

func validateTranferUpdate(transfer models.TransferUpdateBody) error {
	if !slices.Contains(models.TransferStatuses, transfer.Status) {
		return errors.Wrap(
			models.BadParameterError,
			fmt.Sprintf("status %s is not valid", transfer.Status),
		)
	}
	return nil
}

func (usecase *TransferCheckUsecase) QueryTransfers(
	ctx context.Context,
	organizationId string,
	partnerId string,
	clientTransferId string,
) ([]models.Transfer, error) {
	exec := usecase.executorFactory.NewExecutor()

	transferMappings, err := usecase.transferMappingsRepository.ListTransferMappings(
		ctx,
		exec,
		organizationId,
		partnerId,
		clientTransferId)
	if err != nil {
		return []models.Transfer{}, err
	}
	if len(transferMappings) == 0 {
		return make([]models.Transfer, 0), nil
	}

	if err := usecase.enforceSecurity.ReadTransfer(ctx, transferMappings[0]); err != nil {
		logger := utils.LoggerFromContext(ctx)
		logger.ErrorContext(ctx, fmt.Sprintf("Tried to read transfer %s without permission", clientTransferId))
		return make([]models.Transfer, 0), nil
	}

	transfersData, err := usecase.transferDataReader.QueryTransferDataFromMapping(ctx, nil, transferMappings[0])
	if err != nil {
		return nil, err
	}
	if len(transfersData) == 0 {
		return make([]models.Transfer, 0), nil
	}
	beneficiaryInNetwork, err := usecase.beneficiaryIsInNetwork(ctx, transfersData[0])
	if err != nil {
		return nil, err
	}

	transfer := models.Transfer{
		Id:                   transferMappings[0].Id,
		TransferData:         transfersData[0],
		BeneficiaryInNetwork: beneficiaryInNetwork,
	}

	objectId := models.ObjectIdWithPartnerIdPrefix(transferMappings[0].PartnerId, transferMappings[0].ClientTransferId)
	previousDecisions, err := usecase.decisionRepository.DecisionsByObjectId(ctx, exec, organizationId, objectId)
	if err != nil {
		return nil, err
	}
	if len(previousDecisions) > 0 {
		transfer.LastScoredAt = null.TimeFrom(previousDecisions[0].CreatedAt)
		transfer.Score = scoreFromDecision(previousDecisions[0].Score)
	}

	return []models.Transfer{transfer}, nil
}

func (usecase *TransferCheckUsecase) GetTransfer(
	ctx context.Context,
	organizationId string,
	id string,
) (models.Transfer, error) {
	exec := usecase.executorFactory.NewExecutor()

	transferMapping, err := usecase.transferMappingsRepository.GetTransferMapping(ctx, exec, id)
	if err != nil {
		return models.Transfer{}, err
	}

	if err := usecase.enforceSecurity.ReadTransfer(ctx, transferMapping); err != nil {
		return models.Transfer{}, err
	}

	transfersData, err := usecase.transferDataReader.QueryTransferDataFromMapping(ctx, nil, transferMapping)
	if err != nil {
		return models.Transfer{}, err
	}
	if len(transfersData) == 0 {
		return models.Transfer{}, errors.Wrap(
			models.NotFoundError,
			fmt.Sprintf("transfer %s not found", id),
		)
	}
	beneficiaryInNetwork, err := usecase.beneficiaryIsInNetwork(ctx, transfersData[0])
	if err != nil {
		return models.Transfer{}, err
	}

	transfer := models.Transfer{
		Id:                   id,
		TransferData:         transfersData[0],
		BeneficiaryInNetwork: beneficiaryInNetwork,
	}

	objectId := models.ObjectIdWithPartnerIdPrefix(transferMapping.PartnerId, transferMapping.ClientTransferId)
	previousDecisions, err := usecase.decisionRepository.DecisionsByObjectId(ctx, exec, organizationId, objectId)
	if err != nil {
		return models.Transfer{}, err
	}
	if len(previousDecisions) > 0 {
		transfer.LastScoredAt = null.TimeFrom(previousDecisions[0].CreatedAt)
		transfer.Score = scoreFromDecision(previousDecisions[0].Score)
	}
	return transfer, nil
}

func (usecase *TransferCheckUsecase) ScoreTransfer(
	ctx context.Context,
	organizationId string,
	id string,
) (models.Transfer, error) {
	exec := usecase.executorFactory.NewExecutor()

	scenarioId, err := usecase.validateOrgHasTransfercheckEnabled(ctx, organizationId)
	if err != nil {
		return models.Transfer{}, err
	}

	transferMapping, err := usecase.transferMappingsRepository.GetTransferMapping(ctx, exec, id)
	if err != nil {
		return models.Transfer{}, err
	}

	if err := usecase.enforceSecurity.UpdateTransfer(ctx, transferMapping); err != nil {
		return models.Transfer{}, err
	}

	transfersData, err := usecase.transferDataReader.QueryTransferDataFromMapping(ctx, nil, transferMapping)
	if err != nil {
		return models.Transfer{}, err
	}
	if len(transfersData) == 0 {
		return models.Transfer{}, errors.Wrap(
			models.NotFoundError,
			fmt.Sprintf("transfer %s not found", id),
		)
	}
	transferData := transfersData[0]
	beneficiaryInNetwork, err := usecase.beneficiaryIsInNetwork(ctx, transferData)
	if err != nil {
		return models.Transfer{}, err
	}

	_, decision, err := usecase.decisionUseCase.CreateDecision(
		ctx,
		models.CreateDecisionInput{
			ScenarioId: scenarioId,
			ClientObject: &models.ClientObject{
				Data:      transferData.ToIngestionMap(transferMapping),
				TableName: models.TransferCheckTable,
			},
			OrganizationId:     organizationId,
			TriggerObjectTable: models.TransferCheckTable,
		},
		models.CreateDecisionParams{
			WithDecisionWebhooks:        false,
			WithRuleExecutionDetails:    false,
			WithScenarioPermissionCheck: false,
		},
	)
	if err != nil {
		return models.Transfer{}, handleTransferCheckDecisionError(err)
	}

	transfer := models.Transfer{
		Id:                   id,
		TransferData:         transferData,
		LastScoredAt:         null.TimeFrom(decision.CreatedAt),
		Score:                scoreFromDecision(decision.Score),
		BeneficiaryInNetwork: beneficiaryInNetwork,
	}

	return transfer, nil
}

// helper methods
func (usecase *TransferCheckUsecase) validateOrgHasTransfercheckEnabled(ctx context.Context, organizationId string) (scenarioId string, err error) {
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

func (usecase *TransferCheckUsecase) getTransfercheckTable(ctx context.Context,
	organizationId string,
) (models.Table, error) {
	dataModel, err := usecase.dataModelRepository.GetDataModel(
		ctx,
		usecase.executorFactory.NewExecutor(),
		organizationId,
		false,
	)
	if err != nil {
		return models.Table{}, err
	}
	table, ok := dataModel.Tables[models.TransferCheckTable]
	if !ok {
		return models.Table{}, errors.Newf("table %s not found", models.TransferCheckTable)
	}
	return table, nil
}

func scoreFromDecision(score int) null.Int32 {
	if score < 0 {
		score = 0
	} else if score > 100 {
		score = 100
	}

	return null.Int32From(int32(score))
}

func (usecase *TransferCheckUsecase) beneficiaryIsInNetwork(ctx context.Context, transfer models.TransferData) (bool, error) {
	partnersByBic, err := usecase.partnersRepository.ListPartners(
		ctx,
		usecase.executorFactory.NewExecutor(),
		models.PartnerFilters{Bic: null.StringFrom(transfer.BeneficiaryBic)},
	)
	if err != nil {
		return false, errors.Wrap(err, "error while querying partners in TransferCheckUsecase.beneficiaryIsInNetwork")
	}
	if len(partnersByBic) == 0 {
		return false, nil
	}

	return true, nil
}

func handleTransferCheckDecisionError(err error) error {
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return err
	}
	return errors.Wrapf(errors.Handled(err), "Error while creating decision in transfercheck")
}
