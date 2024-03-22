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
	ListTransferMappings(ctx context.Context, exec repositories.Executor, transferId string) ([]models.TransferMapping, error)
	CreateTransferMapping(
		ctx context.Context,
		exec repositories.Executor,
		id string,
		transferMapping models.TransferMappingCreateInput,
	) error
}

type TransferCheckUsecase struct {
	dataModelRepository        repositories.DataModelRepository
	decisionUseCase            DecisionUsecase
	decisionRepository         repositories.DecisionRepository
	executorFactory            executor_factory.ExecutorFactory
	ingestedDataReadRepository repositories.IngestedDataReadRepository
	ingestionRepository        repositories.IngestionRepository
	transactionFactory         executor_factory.TransactionFactory
	transferMappingsRepository transferMappingsRepository
}

const TransferCheckTable = "transfers"

func (usecase *TransferCheckUsecase) CreateTransfer(
	ctx context.Context,
	organizationId string,
	transfer models.TransferCreateBody,
) (models.Transfer, error) {
	logger := utils.LoggerFromContext(ctx)
	exec := usecase.executorFactory.NewExecutor()

	err := validateTransfer(transfer.TransferData)
	if err != nil {
		return models.Transfer{}, err
	}

	dataModel, err := usecase.dataModelRepository.GetDataModel(ctx, exec, organizationId, false)
	if err != nil {
		return models.Transfer{}, err
	}
	table, ok := dataModel.Tables[TransferCheckTable]
	if !ok {
		return models.Transfer{}, errors.Newf("table %s not found", TransferCheckTable)
	}

	clientObject := models.ClientObject{Data: transfer.TransferData.ToMap(), TableName: TransferCheckTable}

	db, err := usecase.executorFactory.NewClientDbExecutor(ctx, organizationId)
	if err != nil {
		return models.Transfer{}, err
	}

	var previousObjects []map[string]interface{}
	previousObjects, err = usecase.ingestedDataReadRepository.QueryIngestedObject(ctx,
		db, table, transfer.TransferData.TransferId)
	if err != nil {
		return models.Transfer{}, err
	}
	if len(previousObjects) > 0 {
		return models.Transfer{}, errors.Wrap(
			models.ConflictError,
			fmt.Sprintf("transfer %s already exists", transfer.TransferData.TransferId),
		)
	}

	var transferMappingId string
	transferMappings, err := usecase.transferMappingsRepository.ListTransferMappings(ctx, exec, transfer.TransferData.TransferId)
	if err != nil {
		return models.Transfer{}, err
	}
	if len(transferMappings) > 0 {
		transferMappingId = transferMappings[0].Id
	} else {
		transferMappingId = uuid.New().String()
		err = usecase.transferMappingsRepository.CreateTransferMapping(ctx, exec,
			transferMappingId, models.TransferMappingCreateInput{
				OrganizationId: organizationId,
				TransferId:     transfer.TransferData.TransferId,
			})
		if err != nil {
			return models.Transfer{}, err
		}
	}

	err = usecase.transactionFactory.TransactionInOrgSchema(ctx, organizationId, func(tx repositories.Executor) error {
		err := usecase.ingestionRepository.IngestObjects(ctx, tx, []models.ClientObject{
			clientObject,
		}, table, logger)
		if err != nil {
			return err
		}

		previousObjects, err = usecase.ingestedDataReadRepository.QueryIngestedObject(ctx,
			tx, table, transfer.TransferData.TransferId)
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return models.Transfer{}, err
	}

	doScore := transfer.SkipScore == nil || !*transfer.SkipScore
	var decision models.DecisionWithRuleExecutions
	if doScore {

		// make a decision
		decision, err = usecase.decisionUseCase.CreateDecision(
			ctx,
			models.CreateDecisionInput{
				ScenarioId:     "585906d7-c55d-44f9-a7bf-b38459ce667d", // TODO: change placeholder
				ClientObject:   clientObject,
				OrganizationId: organizationId,
			},
			logger,
		)
		if err != nil {
			return models.Transfer{}, err
		}
	}

	outTransfer, err := models.TransferFromMap(previousObjects[0])
	if err != nil {
		logger.ErrorContext(ctx, "error while converting transfer from map")
	}

	out := models.Transfer{
		Id:           transferMappingId,
		TransferData: outTransfer,
	}
	if doScore {
		out.LastScoredAt = null.TimeFrom(decision.CreatedAt)
		out.Score = null.Int32From(int32(decision.Score))
	}

	return out, nil
}

func validateTransfer(transfer models.TransferDataCreateBody) error {
	_, err := netip.ParseAddr(transfer.SenderIP)
	if transfer.SenderIP != "" && err != nil {
		return errors.Wrap(models.BadParameterError, "sender_ip is not a valid IP address")
	}

	if !slices.Contains(models.TransferStatuses, transfer.Status) {
		return errors.Wrap(
			models.BadParameterError,
			fmt.Sprintf("status %s is not valid", transfer.Status),
		)
	}

	// TODO implement other validation rules

	return nil
}

func (usecase *TransferCheckUsecase) UpdateTransfer(
	ctx context.Context,
	organizationId string,
	id string,
	transfer models.TransferUpdateBody,
) (models.Transfer, error) {
	logger := utils.LoggerFromContext(ctx)
	exec := usecase.executorFactory.NewExecutor()

	err := validateTranferUpdate(transfer)
	if err != nil {
		return models.Transfer{}, err
	}

	dataModel, err := usecase.dataModelRepository.GetDataModel(ctx, exec, organizationId, false)
	if err != nil {
		return models.Transfer{}, err
	}
	table, ok := dataModel.Tables[TransferCheckTable]
	if !ok {
		return models.Transfer{}, errors.Newf("table %s not found", TransferCheckTable)
	}

	transferMapping, err := usecase.transferMappingsRepository.GetTransferMapping(ctx, exec, id)
	if err != nil {
		return models.Transfer{}, err
	}

	previousDecisions, err := usecase.decisionRepository.DecisionsByObjectId(
		ctx,
		exec,
		organizationId,
		transferMapping.ClientTransferId,
	)
	if err != nil {
		return models.Transfer{}, err
	}

	var previousObjects []map[string]interface{}
	err = usecase.transactionFactory.TransactionInOrgSchema(ctx, organizationId, func(tx repositories.Executor) error {
		previousObjects, err = usecase.ingestedDataReadRepository.QueryIngestedObject(ctx,
			tx, table, transferMapping.ClientTransferId)
		if err != nil {
			return err
		}
		if len(previousObjects) == 0 {
			return errors.Wrap(models.NotFoundError,
				fmt.Sprintf("transfer %s not found", transferMapping.ClientTransferId))
		}

		previousObjects[0]["status"] = transfer.Status
		previousObjects[0]["updated_at"] = time.Now()
		clientObject := models.ClientObject{Data: previousObjects[0], TableName: TransferCheckTable}

		err := usecase.ingestionRepository.IngestObjects(ctx, tx, []models.ClientObject{
			clientObject,
		}, table, logger)
		if err != nil {
			return err
		}

		previousObjects, err = usecase.ingestedDataReadRepository.QueryIngestedObject(
			ctx,
			tx,
			table,
			transferMapping.ClientTransferId,
		)
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return models.Transfer{}, err
	}

	outTransfer, err := models.TransferFromMap(previousObjects[0])
	if err != nil {
		logger.ErrorContext(ctx, "error while converting transfer from map")
	}

	out := models.Transfer{
		Id:           id,
		TransferData: outTransfer,
	}
	if len(previousDecisions) > 0 {
		out.LastScoredAt = null.TimeFrom(previousDecisions[0].CreatedAt)
		out.Score = null.Int32From(int32(previousDecisions[0].Score))
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
	clientTransferId string,
) ([]models.Transfer, error) {
	exec := usecase.executorFactory.NewExecutor()

	dataModel, err := usecase.dataModelRepository.GetDataModel(ctx, exec, organizationId, false)
	if err != nil {
		return nil, err
	}
	table, ok := dataModel.Tables[TransferCheckTable]
	if !ok {
		return nil, errors.Newf("table %s not found", TransferCheckTable)
	}

	transferMappings, err := usecase.transferMappingsRepository.ListTransferMappings(ctx, exec, clientTransferId)
	if err != nil {
		return []models.Transfer{}, err
	}
	if len(transferMappings) == 0 {
		return make([]models.Transfer, 0), nil
	}

	previousDecisions, err := usecase.decisionRepository.DecisionsByObjectId(
		ctx,
		exec,
		organizationId,
		clientTransferId,
	)
	if err != nil {
		return nil, err
	}

	db, err := usecase.executorFactory.NewClientDbExecutor(ctx, organizationId)
	if err != nil {
		return nil, err
	}

	objects, err := usecase.ingestedDataReadRepository.QueryIngestedObject(ctx, db, table, transferMappings[0].ClientTransferId)
	if err != nil {
		return nil, err
	}

	out := make([]models.Transfer, 0)
	if len(objects) > 0 {
		t, err := models.TransferFromMap(objects[0])
		if err != nil {
			return nil, err
		}
		transfer := models.Transfer{
			Id:           transferMappings[0].Id,
			TransferData: t,
		}
		if len(previousDecisions) > 0 {
			transfer.LastScoredAt = null.TimeFrom(previousDecisions[0].CreatedAt)
			transfer.Score = null.Int32From(int32(previousDecisions[0].Score))
		}
		out = append(out, transfer)
	}

	return out, nil
}

func (usecase *TransferCheckUsecase) GetTransfer(
	ctx context.Context,
	organizationId string,
	id string,
) (models.Transfer, error) {
	exec := usecase.executorFactory.NewExecutor()

	dataModel, err := usecase.dataModelRepository.GetDataModel(ctx, exec, organizationId, false)
	if err != nil {
		return models.Transfer{}, err
	}
	table, ok := dataModel.Tables[TransferCheckTable]
	if !ok {
		return models.Transfer{}, errors.Newf("table %s not found", TransferCheckTable)
	}

	transferMapping, err := usecase.transferMappingsRepository.GetTransferMapping(ctx, exec, id)
	if err != nil {
		return models.Transfer{}, err
	}

	previousDecisions, err := usecase.decisionRepository.DecisionsByObjectId(
		ctx,
		exec,
		organizationId,
		transferMapping.ClientTransferId,
	)
	if err != nil {
		return models.Transfer{}, err
	}

	db, err := usecase.executorFactory.NewClientDbExecutor(ctx, organizationId)
	if err != nil {
		return models.Transfer{}, err
	}

	objects, err := usecase.ingestedDataReadRepository.QueryIngestedObject(ctx, db, table, transferMapping.ClientTransferId)
	if err != nil {
		return models.Transfer{}, err
	}

	if len(objects) > 0 {
		t, err := models.TransferFromMap(objects[0])
		if err != nil {
			return models.Transfer{}, err
		}
		transfer := models.Transfer{
			Id:           id,
			TransferData: t,
		}
		if len(previousDecisions) > 0 {
			transfer.LastScoredAt = null.TimeFrom(previousDecisions[0].CreatedAt)
			transfer.Score = null.Int32From(int32(previousDecisions[0].Score))
		}
		return transfer, nil
	}

	return models.Transfer{}, errors.Wrap(
		models.NotFoundError,
		fmt.Sprintf("transfer %s not found", id),
	)
}
