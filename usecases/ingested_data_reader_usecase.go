package usecases

import (
	"context"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
)

type IngestedDataReaderUsecase struct {
	ingestedDataReadRepository repositories.IngestedDataReadRepository
	dataModelRepository        repositories.DataModelRepository
	executorFactory            executor_factory.ExecutorFactory
}

func (usecase *IngestedDataReaderUsecase) GetIngestedObject(ctx context.Context,
	organizationId string, objectType string, objectId string,
) ([]models.DataModelObject, error) {
	exec := usecase.executorFactory.NewExecutor()

	dataModel, err := usecase.dataModelRepository.GetDataModel(ctx, exec, organizationId, true)
	if err != nil {
		return nil, err
	}

	table := dataModel.Tables[objectType]

	db, err := usecase.executorFactory.NewClientDbExecutor(ctx, organizationId)
	if err != nil {
		return nil, err
	}

	return usecase.ingestedDataReadRepository.QueryIngestedObject(ctx, db, table, objectId)
}
