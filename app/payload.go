package app

import (
	"encoding/json"
	"fmt"
	"marble/marble-backend/models"
	"marble/marble-backend/pure_utils"
	"strings"
	"time"

	"github.com/go-playground/validator"
	dynamicstruct "github.com/ompluscator/dynamic-struct"
)

func buildDynamicStruct(fields map[models.FieldName]models.Field) dynamicstruct.DynamicStruct {
	custom_type := dynamicstruct.NewStruct()

	var f = new(float64)
	var i = new(int64)
	var b = new(bool)
	var s = new(string)
	var t = new(time.Time)

	// those fields are mandatory for all tables
	custom_type.AddField("Object_id", s, `validate:"required",json:"object_id"`)
	custom_type.AddField("Updated_at", t, `validate:"required",json:"updated_at"`)

	for fieldName, field := range fields {
		name := string(fieldName)
		switch strings.ToLower(name) {
		case "object_id", "updated_at":
			// already added above, with a different validation tag
		default:
			tag := fmt.Sprintf(`json:"%s"`, name)
			switch field.DataType {
			case models.Bool:
				custom_type.AddField(pure_utils.Capitalize(name), b, tag)
			case models.Int:
				custom_type.AddField(pure_utils.Capitalize(name), i, tag)
			case models.Float:
				custom_type.AddField(pure_utils.Capitalize(name), f, tag)
			case models.String:
				custom_type.AddField(pure_utils.Capitalize(name), s, tag)
			case models.Timestamp:
				custom_type.AddField(pure_utils.Capitalize(name), t, tag)
			}
		}
	}
	return custom_type.Build()
}

func validateParsedJson(instance interface{}) error {
	validate := validator.New()
	err := validate.Struct(instance)
	if err != nil {

		// This error should happen in the dynamic struct is badly formatted, or if the tags
		// contain bad values. If this returns an error, it's a 500 error.
		if _, ok := err.(*validator.InvalidValidationError); ok {
			return err
		}

		// Otherwise it's a 400 error and we can access the reasons from here
		count := 0
		for _, err := range err.(validator.ValidationErrors) {
			fmt.Printf("The input object is not valid: key %v, validation tag: '%v', receive value %v", err.Field(), err.Tag(), err.Param())
			count++
		}
		if count > 0 {
			return models.FormatValidationError
		}
	}
	return nil
}

func ParseToDataModelObject(table models.Table, jsonBody []byte) (models.PayloadReader, error) {
	fields := table.Fields

	custom_type := buildDynamicStruct(fields)

	dynamicStructInstance := custom_type.New()
	dynamicStructReader := dynamicstruct.NewReader(dynamicStructInstance)

	// This is where errors can happen while parson the json. We could for instance have badly formatted
	// json, or timestamps.
	// We could also have more serious errors, like a non-capitalized field in the dynamic struct that
	// causes a panic. We should manage the errors accordingly.
	decoder := json.NewDecoder(strings.NewReader(string(jsonBody)))
	// Reject fields that are not present in the data model/the dynamic struct
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&dynamicStructInstance); err != nil {
		return models.ClientObject{}, fmt.Errorf("%w: %w", models.FormatValidationError, err)
	}

	// If the data has been successfully parsed, we can validate it
	// This is done using the validate tags on the dynamic struct
	// There are two possible cases of error
	err := validateParsedJson(dynamicStructInstance)
	if err != nil {
		return models.ClientObject{}, err
	}

	dataAsMap, err := adaptReaderToMap(dynamicStructReader, table)
	if err != nil {
		return models.ClientObject{}, err
	}

	return models.ClientObject{Data: dataAsMap, TableName: table.Name}, nil
}

func adaptReaderToMap(reader dynamicstruct.Reader, table models.Table) (map[string]any, error) {
	var out = make(map[string]any)

	for fieldName, field := range table.Fields {
		stringFieldName := string(fieldName)
		switch field.DataType {
		case models.Bool:
			if value := reader.GetField(pure_utils.Capitalize(stringFieldName)).PointerBool(); value != nil {
				out[stringFieldName] = *value
			}
		case models.Int:
			if value := reader.GetField(pure_utils.Capitalize(stringFieldName)).PointerInt64(); value != nil {
				out[stringFieldName] = *value
			}
		case models.Float:
			if value := reader.GetField(pure_utils.Capitalize(stringFieldName)).PointerFloat64(); value != nil {
				out[stringFieldName] = *value
			}
		case models.String:
			if value := reader.GetField(pure_utils.Capitalize(stringFieldName)).PointerString(); value != nil {
				out[stringFieldName] = *value
			}
		case models.Timestamp:
			if value := reader.GetField(pure_utils.Capitalize(stringFieldName)).PointerTime(); value != nil {
				out[stringFieldName] = *value
			}
		default:
			return nil, fmt.Errorf("unknown data type %v", field.DataType)
		}
		// if the value was null in the json, add it here to the map
		if _, ok := out[stringFieldName]; !ok {
			out[stringFieldName] = nil
		}
	}

	return out, nil
}
