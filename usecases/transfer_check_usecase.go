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

type enforceSecurityTransferCheck interface {
	CreateTransfer(ctx context.Context, organizationId string, partnerId string) error
	ReadTransfer(ctx context.Context, transferMapping models.TransferMapping) error
	UpdateTransfer(ctx context.Context, transferMapping models.TransferMapping) error
	ReadTransferData(ctx context.Context, partnerId string) error
}

type TransferCheckUsecase struct {
	dataModelRepository        repositories.DataModelRepository
	decisionUseCase            DecisionUsecase
	decisionRepository         repositories.DecisionRepository
	enforceSecurity            enforceSecurityTransferCheck
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
	partnerId string,
	transfer models.TransferCreateBody,
) (models.Transfer, error) {
	logger := utils.LoggerFromContext(ctx)
	exec := usecase.executorFactory.NewExecutor()

	createBody, err := transfer.TransferData.FormatAndValidate()
	if err != nil {
		return models.Transfer{}, err
	}

	scenarioId, err := usecase.validateOrgHasTransfercheckEnabled(ctx, organizationId)
	if err != nil {
		return models.Transfer{}, err
	}

	if err := usecase.enforceSecurity.CreateTransfer(ctx, organizationId, partnerId); err != nil {
		return models.Transfer{}, err
	}

	table, err := usecase.getTransfercheckTable(ctx, organizationId)
	if err != nil {
		return models.Transfer{}, err
	}

	objectId := models.ObjectIdWithPartnerIdPrefix(partnerId, createBody.TransferId)
	previousObjects, err := usecase.lookupPreviousObjects(ctx, nil, organizationId, table, objectId)
	if err != nil {
		return models.Transfer{}, err
	}
	if len(previousObjects) > 0 {
		return models.Transfer{}, errors.Wrap(
			models.ConflictError,
			fmt.Sprintf("transfer %s already exists", createBody.TransferId),
		)
	}

	var transferMappingId string
	transferMappings, err := usecase.transferMappingsRepository.ListTransferMappings(
		ctx,
		exec,
		organizationId,
		partnerId,
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
				PartnerId:        partnerId,
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

	clientObject := models.ClientObject{
		Data:      createBody.ToIngestionMap(transferMappings[0]),
		TableName: TransferCheckTable,
	}

	err = usecase.transactionFactory.TransactionInOrgSchema(ctx, organizationId, func(tx repositories.Executor) error {
		err := usecase.ingestionRepository.IngestObjects(ctx, tx, []models.ClientObject{
			clientObject,
		}, table)
		if err != nil {
			return err
		}

		previousObjects, err = usecase.ingestedDataReadRepository.QueryIngestedObject(ctx, tx, table, objectId)
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		if delErr := usecase.transferMappingsRepository.DeleteTransferMapping(ctx, exec, transferMappingId); delErr != nil {
			logger.ErrorContext(ctx, fmt.Sprintf("error while deleting transfer mapping: %s", delErr.Error()))
		}
		return models.Transfer{}, err
	}

	doScore := transfer.SkipScore == nil || !*transfer.SkipScore
	var decision models.DecisionWithRuleExecutions
	if doScore {
		decision, err = usecase.decisionUseCase.CreateDecision(
			ctx,
			models.CreateDecisionInput{
				ScenarioId:         scenarioId,
				ClientObject:       &clientObject,
				OrganizationId:     organizationId,
				TriggerObjectTable: TransferCheckTable,
			},
			true,
		)
		if err != nil {
			return models.Transfer{}, err
		}
	}

	readPartnerId, transferData := presentTransferData(ctx, previousObjects[0])
	if err := usecase.enforceSecurity.ReadTransferData(ctx, readPartnerId); err != nil {
		return models.Transfer{}, err
	}
	outTransfer, err := models.TransferFromMap(transferData)
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
	var newObjects []map[string]interface{}
	err = usecase.transactionFactory.TransactionInOrgSchema(ctx, organizationId, func(tx repositories.Executor) error {
		previousObjects, err := usecase.lookupPreviousObjects(ctx, tx, organizationId, table, objectId)
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
		}, table)
		if err != nil {
			return err
		}

		newObjects, err = usecase.ingestedDataReadRepository.QueryIngestedObject(ctx, tx, table, objectId)
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return models.Transfer{}, err
	}

	readPartnerId, transferData := presentTransferData(ctx, newObjects[0])
	if err := usecase.enforceSecurity.ReadTransferData(ctx, readPartnerId); err != nil {
		return models.Transfer{}, err
	}
	outTransfer, err := models.TransferFromMap(transferData)
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

	table, err := usecase.getTransfercheckTable(ctx, organizationId)
	if err != nil {
		return nil, err
	}

	objectId := models.ObjectIdWithPartnerIdPrefix(transferMappings[0].PartnerId, transferMappings[0].ClientTransferId)
	objects, err := usecase.lookupPreviousObjects(ctx, nil, organizationId, table, objectId)
	if err != nil {
		return nil, err
	}

	out := make([]models.Transfer, 0)
	if len(objects) > 0 {
		readPartnerId, transferData := presentTransferData(ctx, objects[0])
		if err := usecase.enforceSecurity.ReadTransferData(ctx, readPartnerId); err != nil {
			return nil, err
		}
		outTransfer, err := models.TransferFromMap(transferData)
		if err != nil {
			return nil, err
		}
		transfer := models.Transfer{
			Id:           transferMappings[0].Id,
			TransferData: outTransfer,
		}

		previousDecisions, err := usecase.decisionRepository.DecisionsByObjectId(ctx, exec, organizationId, objectId)
		if err != nil {
			return nil, err
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

	transferMapping, err := usecase.transferMappingsRepository.GetTransferMapping(ctx, exec, id)
	if err != nil {
		return models.Transfer{}, err
	}

	if err := usecase.enforceSecurity.ReadTransfer(ctx, transferMapping); err != nil {
		return models.Transfer{}, err
	}

	table, err := usecase.getTransfercheckTable(ctx, organizationId)
	if err != nil {
		return models.Transfer{}, err
	}

	objectId := models.ObjectIdWithPartnerIdPrefix(transferMapping.PartnerId, transferMapping.ClientTransferId)

	objects, err := usecase.lookupPreviousObjects(ctx, nil, organizationId, table, objectId)
	if err != nil {
		return models.Transfer{}, err
	}

	if len(objects) == 0 {
		return models.Transfer{}, errors.Wrap(
			models.NotFoundError,
			fmt.Sprintf("transfer %s not found", id),
		)
	}

	readPartnerId, transferData := presentTransferData(ctx, objects[0])
	if err := usecase.enforceSecurity.ReadTransferData(ctx, readPartnerId); err != nil {
		return models.Transfer{}, err
	}
	outTransfer, err := models.TransferFromMap(transferData)
	if err != nil {
		return models.Transfer{}, err
	}
	transfer := models.Transfer{
		Id:           id,
		TransferData: outTransfer,
	}

	previousDecisions, err := usecase.decisionRepository.DecisionsByObjectId(ctx, exec, organizationId, objectId)
	if err != nil {
		return models.Transfer{}, err
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

	transferMapping, err := usecase.transferMappingsRepository.GetTransferMapping(ctx, exec, id)
	if err != nil {
		return models.Transfer{}, err
	}

	if err := usecase.enforceSecurity.UpdateTransfer(ctx, transferMapping); err != nil {
		return models.Transfer{}, err
	}

	table, err := usecase.getTransfercheckTable(ctx, organizationId)
	if err != nil {
		return models.Transfer{}, err
	}

	objectId := models.ObjectIdWithPartnerIdPrefix(transferMapping.PartnerId, transferMapping.ClientTransferId)

	objects, err := usecase.lookupPreviousObjects(ctx, nil, organizationId, table, objectId)
	if err != nil {
		return models.Transfer{}, err
	}

	if len(objects) == 0 {
		return models.Transfer{}, errors.Wrap(
			models.NotFoundError,
			fmt.Sprintf("transfer %s not found", id),
		)
	}

	decision, err := usecase.decisionUseCase.CreateDecision(
		ctx,
		models.CreateDecisionInput{
			ScenarioId:     scenarioId,
			ClientObject:   &models.ClientObject{Data: objects[0], TableName: TransferCheckTable},
			OrganizationId: organizationId,
		},
		true,
	)
	if err != nil {
		return models.Transfer{}, err
	}

	readPartnerId, transferData := presentTransferData(ctx, objects[0])
	if err := usecase.enforceSecurity.ReadTransferData(ctx, readPartnerId); err != nil {
		return models.Transfer{}, err
	}
	outTransfer, err := models.TransferFromMap(transferData)
	if err != nil {
		return models.Transfer{}, err
	}
	transfer := models.Transfer{
		Id:           id,
		TransferData: outTransfer,
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

func presentTransferData(ctx context.Context, m map[string]interface{}) (string, map[string]interface{}) {
	const (
		prefixSize    = 36
		separatorSize = 3
	)
	logger := utils.LoggerFromContext(ctx)
	out := make(map[string]interface{})
	for k, v := range m {
		out[k] = v
	}
	objectId, _ := out["object_id"].(string)
	size := len(objectId)
	partnerId := objectId[:min(prefixSize, size)]

	_, err := uuid.Parse(partnerId)
	if err != nil {
		logger.ErrorContext(ctx, fmt.Sprintf("partnerId %s extracted from ingested tranfer is not a valid UUID", partnerId))
		return "", nil
	}

	out["object_id"] = objectId[prefixSize+separatorSize:]
	return partnerId, out
}
