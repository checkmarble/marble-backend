package app

import (
	"context"
	"errors"
)

type DataAccessorImpl struct {
	DataModel  DataModel
	Payload    Payload
	repository RepositoryInterface
}

type DbFieldReadParams struct {
	TriggerTableName TableName
	Path             []LinkName
	FieldName        FieldName
	DataModel        DataModel
	Payload          Payload
}

var ErrNoRowsReadInDB = errors.New("No rows read while reading DB field")

func (d *DataAccessorImpl) GetPayloadField(fieldName string) interface{} {
	return d.Payload.ReadFieldFromPayload(FieldName(fieldName))
}
func (d *DataAccessorImpl) GetDbField(triggerTableName string, path []string, fieldName string) (interface{}, error) {
	return d.repository.GetDbField(context.TODO(), DbFieldReadParams{
		TriggerTableName: TableName(triggerTableName),
		Path:             toLinkNames(path),
		FieldName:        FieldName(fieldName),
		DataModel:        d.DataModel,
		Payload:          d.Payload,
	})
}
