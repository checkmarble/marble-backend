package app

import (
	"encoding/json"
	"fmt"

	"marble/marble-backend/app/data_model"
	"marble/marble-backend/app/dynamic_reading"

	dynamicstruct "github.com/ompluscator/dynamic-struct"
)

func (app *App) ParseToDataModelObject(table data_model.Table, jsonBody []byte) (*dynamic_reading.DynamicStructWithReader, error) {
	fields := table.Fields

	custom_type := dynamic_reading.MakeDynamicStructBuilder(fields)

	dynamicStructInstance := custom_type.New()
	dynamicStructReader := dynamicstruct.NewReader(dynamicStructInstance)

	// This is where errors can happen while parson the json. We could for instance have badly formatted
	// json, or timestamps.
	// We could also have more serious errors, like a non-capitalized field in the dynamic struct that
	// causes a panic. We should manage the errors accordingly.
	err := json.Unmarshal(jsonBody, &dynamicStructInstance)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", dynamic_reading.ErrFormatValidation, err)
	}

	// If the data has been successfully parsed, we can validate it
	// This is done using the validate tags on the dynamic struct
	// There are two possible cases of error
	err = dynamic_reading.ValidateParsedJson(dynamicStructInstance)
	if err != nil {
		return nil, err
	}

	return &dynamic_reading.DynamicStructWithReader{Instance: dynamicStructInstance, Reader: dynamicStructReader, Table: table}, nil
}
