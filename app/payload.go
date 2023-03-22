package app

import (
	"errors"
	"fmt"
)

type Payload struct {
	TableName string
	Data      map[string]interface{}
}

// /////////////////////////////
// Validate payload
// /////////////////////////////

var ErrTriggerObjectAndDataModelMismatch = errors.New("trigger object does not conform to data model")

func (a *App) PayloadFromTriggerObject(organizationID string, triggerObject map[string]any) (Payload, error) {

	// Check that there is a "type" key
	triggerObjectType, found := triggerObject["type"]
	if !found {
		return Payload{}, fmt.Errorf("missing \"type\" key: %w", ErrTriggerObjectAndDataModelMismatch)
	}

	// check that the "type" key is a string
	triggerObjectTypeString, ok := triggerObjectType.(string)
	if !ok {
		return Payload{}, fmt.Errorf("\"type\" key is not a string: %w", ErrTriggerObjectAndDataModelMismatch)
	}

	_, err := a.repository.GetDataModel(organizationID)
	if errors.Is(err, ErrNotFoundInRepository) {
		return Payload{}, fmt.Errorf("data model not found")
	} else if err != nil {
		return Payload{}, fmt.Errorf("error retrieving data model: %w", err)
	}

	// TODO Check the whole data model

	// Data model is validated
	p := Payload{
		TableName: triggerObjectTypeString,
		Data:      triggerObject,
	}

	return p, nil
}
