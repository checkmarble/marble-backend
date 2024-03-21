package usecases

import (
	"context"
	"net/netip"

	"github.com/cockroachdb/errors"
	"github.com/guregu/null/v5"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/checkmarble/marble-backend/utils"
)

type TransferCheckUsecase struct {
	dataModelRepository        repositories.DataModelRepository
	decisionUseCase            DecisionUsecase
	executorFactory            executor_factory.ExecutorFactory
	ingestedDataReadRepository repositories.IngestedDataReadRepository
	ingestionRepository        repositories.IngestionRepository
	transactionFactory         executor_factory.TransactionFactory
}

const TransferCheckTable = "transfers"

func (usecase *TransferCheckUsecase) TransferCheck(
	ctx context.Context,
	organizationId string,
	transfer models.TransferCheckCreateBody,
) (models.TransferCheckResult, error) {
	logger := utils.LoggerFromContext(ctx)
	exec := usecase.executorFactory.NewExecutor()

	err := validateTransfer(transfer)
	if err != nil {
		return models.TransferCheckResult{}, err
	}

	dataModel, err := usecase.dataModelRepository.GetDataModel(ctx, exec, organizationId, false)
	if err != nil {
		return models.TransferCheckResult{}, err
	}
	table, ok := dataModel.Tables[TransferCheckTable]
	if !ok {
		return models.TransferCheckResult{}, errors.Newf("table %s not found", TransferCheckTable)
	}

	clientObject := models.ClientObject{Data: transfer.ToMap(), TableName: TransferCheckTable}

	var previousObjects []map[string]interface{}
	err = usecase.transactionFactory.TransactionInOrgSchema(ctx, organizationId, func(tx repositories.Executor) error {
		err := usecase.ingestionRepository.IngestObjects(ctx, tx, []models.ClientObject{
			clientObject,
		}, table, logger)
		if err != nil {
			return err
		}

		previousObjects, err = usecase.ingestedDataReadRepository.QueryIngestedObject(ctx, tx, table, transfer.TransferId)
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return models.TransferCheckResult{}, err
	}

	// make a decision
	decision, err := usecase.decisionUseCase.CreateDecision(
		ctx,
		models.CreateDecisionInput{
			ScenarioId:     "585906d7-c55d-44f9-a7bf-b38459ce667d", // TODO: change placeholder
			ClientObject:   clientObject,
			OrganizationId: organizationId,
		},
		logger,
	)
	if err != nil {
		return models.TransferCheckResult{}, err
	}

	outTransfer, err := models.TransferFromMap(previousObjects[0])
	if err != nil {
		logger.ErrorContext(ctx, "error while converting transfer from map")
	}

	return models.TransferCheckResult{
		Result: models.TransferCheckScoreDetail{
			Score:        null.Int32From(int32(decision.Score)),
			LastScoredAt: null.TimeFrom(decision.CreatedAt),
		},
		Transfer: outTransfer,
	}, nil
}

func validateTransfer(transfer models.TransferCheckCreateBody) error {
	_, err := netip.ParseAddr(transfer.SenderIP)
	if transfer.SenderIP != "" && err != nil {
		return errors.Wrap(models.BadParameterError, "sender_ip is not a valid IP address")
	}

	// TODO implement other validation rules

	return nil
}
