package app

import (
	"context"
	"errors"
	"fmt"
	"marble/marble-backend/app/operators"
)

type DataAccessorImpl struct {
	DataModel  DataModel
	Payload    DynamicStructWithReader
	repository RepositoryInterface
}

type DbFieldReadParams struct {
	Path      []string
	FieldName string
	DataModel DataModel
	Payload   DynamicStructWithReader
}

var ErrNoRowsReadInDB = errors.New("No rows read while reading DB field")

func (d *DataAccessorImpl) GetPayloadField(fieldName string) interface{} {
	return d.GetPayloadField(fieldName)
}
func (d *DataAccessorImpl) GetDbField(path []string, fieldName string) (interface{}, error) {
	return d.repository.GetDbField(context.TODO(), DbFieldReadParams{
		Path:      path,
		FieldName: fieldName,
		DataModel: d.DataModel,
		Payload:   d.Payload,
	})
}

func (d *DataAccessorImpl) ValidateDbFieldReadConsistency(path []string, fieldName string) error {
	if len(path) == 0 {
		return fmt.Errorf("Path is empty: %w", operators.ErrDbReadInconsistentWithDataModel)
	}

	for _, tableName := range path {
		_, ok := d.DataModel.Tables[tableName]
		if !ok {
			return fmt.Errorf("Table %s in path not found in data model: %w", tableName, operators.ErrDbReadInconsistentWithDataModel)
		}
	}

	lastTable := d.DataModel.Tables[path[len(path)-1]]
	_, ok := lastTable.Fields[fieldName]
	if !ok {
		return fmt.Errorf("Field %s in table %s not found in data model: %w", fieldName, lastTable.Name, operators.ErrDbReadInconsistentWithDataModel)
	}

	return nil
}
