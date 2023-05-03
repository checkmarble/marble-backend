package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"

	"github.com/go-playground/validator"
	dynamicstruct "github.com/ompluscator/dynamic-struct"
)

func capitalize(str string) string {
	runes := []rune(str)
	runes[0] = unicode.ToUpper(runes[0])
	return string(runes)
}

type DynamicStructWithReader struct {
	Instance interface{}
	Reader   dynamicstruct.Reader
	Table    Table
}

var ErrFormatValidation = errors.New("The input object is not valid")

var validate *validator.Validate

func makeDynamicStructBuilder(fields map[string]Field) dynamicstruct.DynamicStruct {
	custom_type := dynamicstruct.NewStruct()

	var stringPointerType *string
	var intPointerType *int
	var floatPointerType *float32
	var boolPointerType *bool
	var timePointerType *time.Time

	// those fields are mandatory for all tables
	custom_type.AddField("Object_id", stringPointerType, `validate:"required"`)
	custom_type.AddField("Updated_at", timePointerType, `validate:"required"`)

	for fieldName, field := range fields {
		switch strings.ToLower(fieldName) {
		case "object_id", "updated_at":
			// already added above, with a different validation tag
			break
		default:
			switch field.DataType {
			case Bool:
				custom_type.AddField(capitalize(fieldName), boolPointerType, "")
			case Int:
				custom_type.AddField(capitalize(fieldName), intPointerType, "")
			case Float:
				custom_type.AddField(capitalize(fieldName), floatPointerType, "")
			case String:
				custom_type.AddField(capitalize(fieldName), stringPointerType, "")
			case Timestamp:
				custom_type.AddField(capitalize(fieldName), timePointerType, "")
			}
		}
	}
	return custom_type.Build()
}

func validateParsedJson(instance interface{}) error {
	validate = validator.New()
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
			return ErrFormatValidation
		}
	}
	return nil
}

func ParseToDataModelObject(_ context.Context, table Table, jsonBody []byte) (DynamicStructWithReader, error) {
	fields := table.Fields

	custom_type := makeDynamicStructBuilder(fields)

	dynamicStructInstance := custom_type.New()
	dynamicStructReader := dynamicstruct.NewReader(dynamicStructInstance)

	// This is where errors can happen while parson the json. We could for instance have badly formatted
	// json, or timestamps.
	// We could also have more serious errors, like a non-capitalized field in the dynamic struct that
	// causes a panic. We should manage the errors accordingly.
	err := json.Unmarshal(jsonBody, &dynamicStructInstance)
	if err != nil {
		return DynamicStructWithReader{}, fmt.Errorf("%w: %w", ErrFormatValidation, err)
	}

	// If the data has been successfully parsed, we can validate it
	// This is done using the validate tags on the dynamic struct
	// There are two possible cases of error
	err = validateParsedJson(dynamicStructInstance)
	if err != nil {
		return DynamicStructWithReader{}, err
	}

	return DynamicStructWithReader{Instance: dynamicStructInstance, Reader: dynamicStructReader, Table: table}, nil
}

func (dynamicStruct DynamicStructWithReader) ReadFieldFromDynamicStruct(fieldName string) interface{} {
	field := dynamicStruct.Reader.GetField(capitalize(fieldName))
	table := dynamicStruct.Table
	fields := table.Fields
	fieldFromModel := fields[fieldName]

	switch fieldFromModel.DataType {
	case Bool:
		return field.PointerBool()
	case Int:
		return field.PointerInt()
	case Float:
		return field.PointerFloat32()
	case String:
		return field.PointerString()
	case Timestamp:
		return field.PointerTime()
	default:
		panic("Unknown data type")
	}
}
