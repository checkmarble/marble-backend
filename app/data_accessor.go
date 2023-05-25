package app

import (
	"context"
	"marble/marble-backend/models"
)

type DataAccessorImpl struct {
	DataModel  models.DataModel
	Payload    Payload
	repository RepositoryInterface
}

type DbFieldReadParams struct {
	TriggerTableName models.TableName
	Path             []models.LinkName
	FieldName        models.FieldName
	DataModel        models.DataModel
	Payload          Payload
}

func (d *DataAccessorImpl) GetPayloadField(fieldName string) (interface{}, error) {
	return d.Payload.ReadFieldFromPayload(models.FieldName(fieldName))
}
func (d *DataAccessorImpl) GetDbField(triggerTableName string, path []string, fieldName string) (interface{}, error) {
	return d.repository.GetDbField(context.TODO(), DbFieldReadParams{
		TriggerTableName: models.TableName(triggerTableName),
		Path:             models.ToLinkNames(path),
		FieldName:        models.FieldName(fieldName),
		DataModel:        d.DataModel,
		Payload:          d.Payload,
	})
}
