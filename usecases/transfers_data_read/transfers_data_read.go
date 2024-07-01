package transfers_data_read

import (
	"context"
	"fmt"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
)

type enforceSecurity interface {
	ReadTransferData(ctx context.Context, partnerId string) error
}

type TransferDataReader struct {
	enforceSecurity            enforceSecurity
	executorFactory            executor_factory.ExecutorFactory
	ingestedDataReadRepository repositories.IngestedDataReadRepository
	dataModelRepository        repositories.DataModelRepository
}

func NewTransferDataReader(
	enforceSecurity enforceSecurity,
	executorFactory executor_factory.ExecutorFactory,
	ingestedDataReadRepository repositories.IngestedDataReadRepository,
	dataModelRepository repositories.DataModelRepository,
) TransferDataReader {
	return TransferDataReader{
		enforceSecurity:            enforceSecurity,
		executorFactory:            executorFactory,
		ingestedDataReadRepository: ingestedDataReadRepository,
		dataModelRepository:        dataModelRepository,
	}
}

func (usecase TransferDataReader) QueryTransferDataFromMapping(
	ctx context.Context,
	db repositories.Executor,
	transferMapping models.TransferMapping,
) ([]models.TransferData, error) {
	if db == nil {
		var err error
		db, err = usecase.executorFactory.NewClientDbExecutor(ctx, transferMapping.OrganizationId)
		if err != nil {
			return nil, err
		}
	}

	exec := usecase.executorFactory.NewExecutor()
	dataModel, err := usecase.dataModelRepository.GetDataModel(ctx, exec, transferMapping.OrganizationId, false)
	if err != nil {
		return nil, err
	}
	table, ok := dataModel.Tables[models.TransferCheckTable]
	if !ok {
		return nil, errors.Newf("table %s not found", models.TransferCheckTable)
	}

	objectId := models.ObjectIdWithPartnerIdPrefix(transferMapping.PartnerId, transferMapping.ClientTransferId)
	objects, err := usecase.ingestedDataReadRepository.QueryIngestedObject(ctx, db, table, objectId)
	if err != nil {
		return nil, errors.Wrap(err, "error while querying ingested objects in TransferDataReader.GetTransferById")
	}

	if len(objects) == 0 {
		return make([]models.TransferData, 0), nil
	}

	readPartnerId, transferData := presentTransferData(ctx, objects[0])
	if err := usecase.enforceSecurity.ReadTransferData(ctx, readPartnerId); err != nil {
		return nil, err
	}

	out, err := models.TransferFromMap(transferData)
	return []models.TransferData{out}, err
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
