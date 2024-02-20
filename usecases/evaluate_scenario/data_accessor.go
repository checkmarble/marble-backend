package evaluate_scenario

import (
	"context"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
)

type DataAccessor struct {
	DataModel                  models.DataModel
	ClientObject               models.ClientObject
	executorFactory            executor_factory.ExecutorFactory
	organizationId             string
	ingestedDataReadRepository repositories.IngestedDataReadRepository
}

func (d *DataAccessor) GetDbField(ctx context.Context, triggerTableName string, path []string, fieldName string) (interface{}, error) {
	db, err := d.executorFactory.NewClientDbExecutor(ctx, d.organizationId)
	if err != nil {
		return nil, err
	}
	return d.ingestedDataReadRepository.GetDbField(
		ctx,
		db,
		models.DbFieldReadParams{
			TriggerTableName: models.TableName(triggerTableName),
			Path:             models.ToLinkNames(path),
			FieldName:        models.FieldName(fieldName),
			DataModel:        d.DataModel,
			ClientObject:     d.ClientObject,
		})
}
