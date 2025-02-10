package models

import (
	"encoding/json"
)

type DbFieldReadParams struct {
	TriggerTableName string
	Path             []string
	FieldName        string
	DataModel        DataModel
	ClientObject     ClientObject
}

type MissingField struct {
	Field          Field
	ErrorIfMissing string
}

type ClientObject struct {
	TableName string
	Data      map[string]any

	// MissingFieldsToLookup is a list of fields that are missing from the payload but exist in the data model.
	// It is used in the context of partial updates to fetch the missing fields from the database.
	// It is not related to whether the field is actually required in the data model or not.
	MissingFieldsToLookup []MissingField
}

// expects format {"field_name": "error message", ...}
type IngestionValidationErrorsSingle map[string]string

func (err IngestionValidationErrorsSingle) Error() string {
	encoded, _ := json.Marshal(err)
	return string(encoded)
}

func (err IngestionValidationErrorsSingle) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]string(err))
}

// expects format {"object_id": {"field_name": "error message"}, ...}
type IngestionValidationErrorsMultiple map[string]map[string]string

func (err IngestionValidationErrorsMultiple) Error() string {
	encoded, _ := json.Marshal(err)
	return string(encoded)
}

func (err IngestionValidationErrorsMultiple) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]map[string]string(err))
}
