package models

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
	"unicode"

	"github.com/go-playground/validator"
	dynamicstruct "github.com/ompluscator/dynamic-struct"
)

type PayloadReader interface {
	ReadFieldFromPayload(fieldName FieldName) (any, error)
	ReadTableName() TableName
}

type DbFieldReadParams struct {
	TriggerTableName TableName
	Path             []LinkName
	FieldName        FieldName
	DataModel        DataModel
	Payload          PayloadReader
}

type Payload struct {
	Reader    dynamicstruct.Reader
	TableName TableName
}

func (payload Payload) ReadFieldFromPayload(fieldName FieldName) (any, error) {
	// output type is string, bool, float64, int64, time.Time, bundled in an "any" interface
	field := payload.Reader.GetField(capitalize(string(fieldName)))

	return field.Interface(), nil
}

func (payload Payload) ReadTableName() TableName {
	return payload.TableName
}

type ClientObject struct {
	TableName TableName
	Data      map[string]any
}

func (obj ClientObject) ReadFieldFromPayload(fieldName FieldName) (any, error) {
	// output type is string, bool, float64, int64, time.Time, bundled in an "any" interface
	fieldValue, ok := obj.Data[string(fieldName)]
	if !ok {
		return nil, fmt.Errorf("No field with name %s", fieldName)
	}
	return fieldValue, nil
}

func (obj ClientObject) ReadTableName() TableName {
	return obj.TableName
}

func buildDynamicStruct(fields map[FieldName]Field) dynamicstruct.DynamicStruct {
	custom_type := dynamicstruct.NewStruct()

	var f float64
	var i int64

	// those fields are mandatory for all tables
	custom_type.AddField("Object_id", "", `validate:"required"`)
	custom_type.AddField("Updated_at", time.Time{}, `validate:"required"`)

	for fieldName, field := range fields {
		name := string(fieldName)
		switch strings.ToLower(name) {
		case "object_id", "updated_at":
			// already added above, with a different validation tag
			break
		default:
			switch field.DataType {
			case Bool:
				custom_type.AddField(capitalize(name), true, "")
			case Int:
				custom_type.AddField(capitalize(name), i, "")
			case Float:
				custom_type.AddField(capitalize(name), f, "")
			case String:
				custom_type.AddField(capitalize(name), "", "")
			case Timestamp:
				custom_type.AddField(capitalize(name), time.Time{}, "")
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
			return FormatValidationError
		}
	}
	return nil
}

func capitalize(str string) string {
	runes := []rune(str)
	runes[0] = unicode.ToUpper(runes[0])
	return string(runes)
}


func ParseToDataModelObject(table Table, jsonBody []byte) (Payload, error) {
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
		return Payload{}, fmt.Errorf("%w: %w", FormatValidationError, err)
	}

	// If the data has been successfully parsed, we can validate it
	// This is done using the validate tags on the dynamic struct
	// There are two possible cases of error
	err := validateParsedJson(dynamicStructInstance)
	if err != nil {
		return Payload{}, err
	}

	return Payload{Reader: dynamicStructReader, TableName: table.Name}, nil
}
