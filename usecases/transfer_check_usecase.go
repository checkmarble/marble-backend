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
	ListTransferMappings(ctx context.Context, exec repositories.Executor, clientTransferId string) ([]models.TransferMapping, error)
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
	organizationRepository     repositories.OrganizationRepository
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

	if err := validateTransfer(transfer.TransferData); err != nil {
		return models.Transfer{}, err
	}

	scenarioId, err := usecase.validateOrgHasTransfercheckEnabled(ctx, organizationId)
	if err != nil {
		return models.Transfer{}, err
	}

	table, err := usecase.getTransfercheckTable(ctx, organizationId)
	if err != nil {
		return models.Transfer{}, err
	}

	clientObject := models.ClientObject{Data: transfer.TransferData.ToMap(), TableName: TransferCheckTable}

	previousObjects, err := usecase.lookupPreviousObjects(ctx, nil, organizationId, table, transfer.TransferData.TransferId)
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
				OrganizationId:   organizationId,
				ClientTransferId: transfer.TransferData.TransferId,
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
		decision, err = usecase.decisionUseCase.CreateDecision(
			ctx,
			models.CreateDecisionInput{
				ScenarioId:     scenarioId,
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

	table, err := usecase.getTransfercheckTable(ctx, organizationId)
	if err != nil {
		return models.Transfer{}, err
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

	var newObjects []map[string]interface{}
	err = usecase.transactionFactory.TransactionInOrgSchema(ctx, organizationId, func(tx repositories.Executor) error {
		previousObjects, err := usecase.lookupPreviousObjects(ctx, tx, organizationId,
			table, transferMapping.ClientTransferId)
		if err != nil {
			return err
		}
		if len(previousObjects) == 0 {
			return errors.Wrap(
				models.NotFoundError,
				fmt.Sprintf("transfer %s not found", transferMapping.ClientTransferId),
			)
		}

		previousObjects[0]["status"] = transfer.Status
		previousObjects[0]["updated_at"] = time.Now()
		clientObject := models.ClientObject{Data: previousObjects[0], TableName: TransferCheckTable}

		err = usecase.ingestionRepository.IngestObjects(ctx, tx, []models.ClientObject{
			clientObject,
		}, table, logger)
		if err != nil {
			return err
		}

		newObjects, err = usecase.ingestedDataReadRepository.QueryIngestedObject(
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

	outTransfer, err := models.TransferFromMap(newObjects[0])
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

	table, err := usecase.getTransfercheckTable(ctx, organizationId)
	if err != nil {
		return nil, err
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

	objects, err := usecase.lookupPreviousObjects(ctx, nil, organizationId, table, transferMappings[0].ClientTransferId)
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

	table, err := usecase.getTransfercheckTable(ctx, organizationId)
	if err != nil {
		return models.Transfer{}, err
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

	objects, err := usecase.lookupPreviousObjects(ctx, nil, organizationId, table, transferMapping.ClientTransferId)
	if err != nil {
		return models.Transfer{}, err
	}

	if len(objects) == 0 {
		return models.Transfer{}, errors.Wrap(
			models.NotFoundError,
			fmt.Sprintf("transfer %s not found", id),
		)
	}

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

	table, err := usecase.getTransfercheckTable(ctx, organizationId)
	if err != nil {
		return models.Transfer{}, err
	}

	transferMapping, err := usecase.transferMappingsRepository.GetTransferMapping(ctx, exec, id)
	if err != nil {
		return models.Transfer{}, err
	}

	objects, err := usecase.lookupPreviousObjects(ctx, nil, organizationId, table, transferMapping.ClientTransferId)
	if err != nil {
		return models.Transfer{}, err
	}

	if len(objects) == 0 {
		return models.Transfer{}, errors.Wrap(
			models.NotFoundError,
			fmt.Sprintf("transfer %s not found", id),
		)
	}

	t, err := models.TransferFromMap(objects[0])
	if err != nil {
		return models.Transfer{}, err
	}
	transfer := models.Transfer{
		Id:           id,
		TransferData: t,
	}

	decision, err := usecase.decisionUseCase.CreateDecision(
		ctx,
		models.CreateDecisionInput{
			ScenarioId:     scenarioId,
			ClientObject:   models.ClientObject{Data: objects[0], TableName: TransferCheckTable},
			OrganizationId: organizationId,
		},
		utils.LoggerFromContext(ctx),
	)
	if err != nil {
		return models.Transfer{}, err
	}
	transfer.LastScoredAt = null.TimeFrom(decision.CreatedAt)
	transfer.Score = null.Int32From(int32(decision.Score))

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
	table, ok := dataModel.Tables[TransferCheckTable]
	if !ok {
		return models.Table{}, errors.Newf("table %s not found", TransferCheckTable)
	}
	return table, nil
}

func (usecase *TransferCheckUsecase) lookupPreviousObjects(
	ctx context.Context,
	exec repositories.Executor,
	organizationId string,
	table models.Table,
	clientTransferId string,
) ([]map[string]interface{}, error) {
	if exec == nil {
		db, err := usecase.executorFactory.NewClientDbExecutor(ctx, organizationId)
		if err != nil {
			return nil, err
		}
		exec = db
	}
	objects, err := usecase.ingestedDataReadRepository.QueryIngestedObject(ctx, exec, table, clientTransferId)
	if err != nil {
		return nil, errors.Wrap(err, "error while querying ingested objects in lookupPreviousObjects")
	}
	return objects, nil
}

// other helper functions
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
