package app

import (
	"context"
	"marble/marble-backend/models"

	"github.com/jackc/pgx/v5/pgxpool"
)

type DataAccessorImpl struct {
	DataModel  models.DataModel
	Payload    models.Payload
	repository RepositoryInterface
}

func (d *DataAccessorImpl) GetPayloadField(fieldName string) (interface{}, error) {
	return d.Payload.ReadFieldFromPayload(models.FieldName(fieldName))
}
func (d *DataAccessorImpl) GetDbField(triggerTableName string, path []string, fieldName string) (interface{}, error) {
	return d.repository.GetDbField(context.TODO(), models.DbFieldReadParams{
		TriggerTableName: models.TableName(triggerTableName),
		Path:             models.ToLinkNames(path),
		FieldName:        models.FieldName(fieldName),
		DataModel:        d.DataModel,
		Payload:          d.Payload,
	})
}
func (d *DataAccessorImpl) GetDbHandle() *pgxpool.Pool {
	return nil
}
