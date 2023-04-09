package app

import (
	"fmt"
	"marble/marble-backend/app/operators"
)

type DataAccessorImpl struct {
	DataModel  DataModel
	Payload    Payload
	repository RepositoryInterface
}

func (d *DataAccessorImpl) GetPayloadField(fieldName string) (interface{}, error) {
	return nil, nil
}
func (d *DataAccessorImpl) GetDbField(path []string, fieldName string) (interface{}, error) {
	return d.repository.GetDbField(path, fieldName, d.DataModel, d.Payload)
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
