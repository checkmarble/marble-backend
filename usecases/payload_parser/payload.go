package payload_parser

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/go-playground/validator/v10"
	dynamicstruct "github.com/ompluscator/dynamic-struct"
	"github.com/tidwall/gjson"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
)

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

func buildDynamicStruct(fields map[models.FieldName]models.Field) dynamicstruct.DynamicStruct {
	customType := dynamicstruct.NewStruct()

	var f = new(float64)
	var i = new(int64)
	var b = new(bool)
	var s = new(string)
	var t = new(time.Time)

	// those fields are mandatory for all tables
	customType.AddField("Object_id", s, `validate:"required",json:"object_id"`)
	customType.AddField("Updated_at", t, `validate:"required",json:"updated_at"`)

	for fieldName, field := range fields {
		name := string(fieldName)
		switch strings.ToLower(name) {
		case "object_id", "updated_at":
			// already added above, with a different validation tag
		default:
			tag := fmt.Sprintf(`json:"%s"`, name)
			switch field.DataType {
			case models.Bool:
				customType.AddField(pure_utils.Capitalize(name), b, tag)
			case models.Int:
				customType.AddField(pure_utils.Capitalize(name), i, tag)
			case models.Float:
				customType.AddField(pure_utils.Capitalize(name), f, tag)
			case models.String:
				customType.AddField(pure_utils.Capitalize(name), s, tag)
			case models.Timestamp:
				customType.AddField(pure_utils.Capitalize(name), t, tag)
			}
		}
	}
	return customType.Build()
}

func ParseToDataModelObject(table models.Table, jsonBody []byte) (models.PayloadReader, error) {
	fields := table.Fields

	customType := buildDynamicStruct(fields)

	dynamicStructInstance := customType.New()
	dynamicStructReader := dynamicstruct.NewReader(dynamicStructInstance)

	// This is where errors can happen while parson the json. We could for instance have badly formatted
	// json, or timestamps.
	// We could also have more serious errors, like a non-capitalized field in the dynamic struct that
	// causes a panic. We should manage the errors accordingly.
	decoder := json.NewDecoder(bytes.NewReader(jsonBody))
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

	for fieldName := range table.Fields {
		stringFieldName := string(fieldName)

		// dynamicstruct return pointers (*time.Time)
		pointer := reader.GetField(pure_utils.Capitalize(stringFieldName)).Interface()

		reflectValue := reflect.ValueOf(pointer)
		if reflectValue.IsNil() {
			out[stringFieldName] = nil
		} else {
			// dereference the pointer (Indirect) then take the value
			out[stringFieldName] = reflect.Indirect(reflectValue).Interface()
		}
	}

	return out, nil
}

type fieldValidator map[models.DataType]func(result gjson.Result) error

type Parser struct {
	validators fieldValidator
}

var errIsInvalidJSON = fmt.Errorf("json is invalid")
var errIsNotNullable = fmt.Errorf("is not nullable")
var errIsInvalidTimestamp = fmt.Errorf("is not a valid timestamp")
var errIsInvalidInteger = fmt.Errorf("is not a valid integer")
var errIsInvalidFloat = fmt.Errorf("is not a valid float")
var errIsInvalidBoolean = fmt.Errorf("is not a valid boolean")
var errIsInvalidString = fmt.Errorf("is not a valid string")
var errIsInvalidDataType = fmt.Errorf("invalid type")

func (p *Parser) ValidatePayload(table models.Table, json []byte) (map[models.FieldName]string, error) {
	if !gjson.ValidBytes(json) {
		return nil, errIsInvalidJSON
	}

	errors := make(map[models.FieldName]string)
	result := gjson.ParseBytes(json)
	for name, field := range table.Fields {
		value := result.Get(string(name))
		if !value.Exists() {
			if !field.Nullable {
				errors[name] = errIsNotNullable.Error()
			}
			continue
		}

		validateField, ok := p.validators[field.DataType]
		if !ok {
			return nil, fmt.Errorf("%w: %s", errIsInvalidDataType, field.DataType.String())
		}
		if err := validateField(value); err != nil {
			errors[name] = err.Error()
		}
	}
	if len(errors) > 0 {
		return errors, nil
	}
	return nil, nil
}

func NewParser() *Parser {
	validators := fieldValidator{
		models.Timestamp: func(result gjson.Result) error {
			_, err := time.Parse(time.RFC3339, result.String())
			if err != nil {
				return fmt.Errorf("%w: expected format YYYY-MM-DDThh:mm:ss[+optional decimals]Z, got %s", errIsInvalidTimestamp, result.String())
			}
			return nil
		},
		models.Int: func(result gjson.Result) error {
			_, err := strconv.ParseInt(result.Raw, 10, 64)
			if err != nil {
				return fmt.Errorf("%w: expected an integer, got %s", errIsInvalidInteger, result.Raw)
			}
			return nil
		},
		models.Float: func(result gjson.Result) error {
			_, err := strconv.ParseFloat(result.Raw, 64)
			if err != nil {
				return fmt.Errorf("%w: expected a float, got %s", errIsInvalidFloat, result.Raw)
			}
			return nil
		},
		models.String: func(result gjson.Result) error {
			if result.Type != gjson.String {
				return errIsInvalidString
			}
			return nil
		},
		models.Bool: func(result gjson.Result) error {
			if !result.IsBool() {
				return fmt.Errorf("%w: expected a boolean, got %s", errIsInvalidBoolean, result.Raw)
			}
			return nil
		},
	}

	return &Parser{
		validators: validators,
	}
}
