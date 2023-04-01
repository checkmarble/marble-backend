package dynamic_reading

import (
	"errors"
	"fmt"
	"log"
	"strings"
	"time"
	"unicode"

	"marble/marble-backend/app/data_model"
	payload_package "marble/marble-backend/app/payload"

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
	Table    data_model.Table
}

var ErrFormatValidation = errors.New("The input object is not valid")

var validate *validator.Validate

func MakeDynamicStructBuilder(fields map[string]data_model.Field) dynamicstruct.DynamicStruct {
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
			case data_model.Bool:
				custom_type.AddField(capitalize(fieldName), boolPointerType, "")
			case data_model.Int:
				custom_type.AddField(capitalize(fieldName), intPointerType, "")
			case data_model.Float:
				custom_type.AddField(capitalize(fieldName), floatPointerType, "")
			case data_model.String:
				custom_type.AddField(capitalize(fieldName), stringPointerType, "")
			case data_model.Timestamp:
				custom_type.AddField(capitalize(fieldName), timePointerType, "")
			}
		}
	}
	return custom_type.Build()
}

func ValidateParsedJson(instance interface{}) error {
	validate = validator.New()
	err := validate.Struct(instance)
	if err != nil {

		// This error should happen in the dynamic struct is badly formatted, or if the tags
		// contain bad values. If this returns an error, it's a 500 error.
		if _, ok := err.(*validator.InvalidValidationError); ok {
			log.Println(err)
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

func (dynamicStruct DynamicStructWithReader) ReadFieldFromDynamicStruct(fieldName string) interface{} {
	check := dynamicStruct.Reader.HasField((capitalize(fieldName)))
	if !check {
		log.Fatalf("Field %v not found in dynamic struct", fieldName)
	}
	field := dynamicStruct.Reader.GetField(capitalize(fieldName))
	table := dynamicStruct.Table
	fields := table.Fields
	fieldFromModel, ok := fields[fieldName]
	if !ok {
		log.Fatalf("Field %v not found in table when reading from dynamic struct", fieldName)
	}

	switch fieldFromModel.DataType {
	case data_model.Bool:
		return field.PointerBool()
	case data_model.Int:
		return field.PointerInt()
	case data_model.Float:
		return field.PointerFloat32()
	case data_model.String:
		return field.PointerString()
	case data_model.Timestamp:
		return field.PointerTime()
	default:
		log.Fatalf("Unknown data type: %s", fieldFromModel.DataType)
		return nil
	}
}

type EvaluationContext struct {
	Payload             payload_package.Payload
	TriggerRequirements map[string]DBEntities
	RulesRequirements   map[string]DBEntities
}

type DBEntities struct {
	TablePath string
	Table     data_model.Table
	Reader    dynamicstruct.Reader
}
