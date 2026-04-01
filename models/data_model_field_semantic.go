package models

import (
	"fmt"
	"slices"

	"github.com/cockroachdb/errors"
)

///////////////////////////////
// Field Semantic Type
///////////////////////////////

type FieldSemanticType string

const (
	FieldSemanticTypeUnset FieldSemanticType = ""

	// Name family
	FieldSemanticTypeName       FieldSemanticType = "name"
	FieldSemanticTypeFirstName  FieldSemanticType = "first_name"
	FieldSemanticTypeMiddleName FieldSemanticType = "middle_name"
	FieldSemanticTypeLastName   FieldSemanticType = "last_name"

	// Unique ID family
	FieldSemanticTypeRegistrationNumber FieldSemanticType = "registration_number"
	FieldSemanticTypeTaxId              FieldSemanticType = "tax_id"
	FieldSemanticTypeOpaqueId           FieldSemanticType = "opaque_id"
	FieldSemanticTypeIban               FieldSemanticType = "iban"
	FieldSemanticTypeAccountNumber      FieldSemanticType = "account_number"
	FieldSemanticTypeBic                FieldSemanticType = "bic"

	// URL family
	FieldSemanticTypeEmail       FieldSemanticType = "email"
	FieldSemanticTypeUrl         FieldSemanticType = "url"
	FieldSemanticTypePhoneNumber FieldSemanticType = "phone_number"

	// Time family
	FieldSemanticTypeBirthDate FieldSemanticType = "birth_date"
	FieldSemanticTypeCreatedAt FieldSemanticType = "created_at"
	FieldSemanticTypeUpdatedAt FieldSemanticType = "updated_at"
	FieldSemanticTypeDeletedAt FieldSemanticType = "deleted_at"

	// Enum family
	FieldSemanticTypeCurrency FieldSemanticType = "currency"
	FieldSemanticTypeCountry  FieldSemanticType = "country"
	FieldSemanticTypeMccCode  FieldSemanticType = "mcc_code"

	// Number family
	FieldSemanticTypeAmount     FieldSemanticType = "amount"
	FieldSemanticTypePercentage FieldSemanticType = "percentage"
	FieldSemanticTypeQuantity   FieldSemanticType = "quantity"
)

type fieldSemanticTypeValidator interface {
	AllowedDataTypes() []DataType
}

type stringSemanticType struct{}

func (stringSemanticType) AllowedDataTypes() []DataType { return []DataType{String} }

type numberSemanticType struct{}

func (numberSemanticType) AllowedDataTypes() []DataType { return []DataType{Int, Float} }

type timestampSemanticType struct{}

func (timestampSemanticType) AllowedDataTypes() []DataType { return []DataType{Timestamp} }

var fieldSemanticTypeRegistry = map[FieldSemanticType]fieldSemanticTypeValidator{
	// Name family
	FieldSemanticTypeName:       stringSemanticType{},
	FieldSemanticTypeFirstName:  stringSemanticType{},
	FieldSemanticTypeMiddleName: stringSemanticType{},
	FieldSemanticTypeLastName:   stringSemanticType{},

	// Unique ID family
	FieldSemanticTypeRegistrationNumber: stringSemanticType{},
	FieldSemanticTypeTaxId:              stringSemanticType{},
	FieldSemanticTypeOpaqueId:           stringSemanticType{},
	FieldSemanticTypeIban:               stringSemanticType{},
	FieldSemanticTypeAccountNumber:      stringSemanticType{},
	FieldSemanticTypeBic:                stringSemanticType{},

	// URL family
	FieldSemanticTypeEmail:       stringSemanticType{},
	FieldSemanticTypeUrl:         stringSemanticType{},
	FieldSemanticTypePhoneNumber: stringSemanticType{},

	// Time family
	FieldSemanticTypeBirthDate: timestampSemanticType{},
	FieldSemanticTypeCreatedAt: timestampSemanticType{},
	FieldSemanticTypeUpdatedAt: timestampSemanticType{},
	FieldSemanticTypeDeletedAt: timestampSemanticType{},

	// Enum family
	FieldSemanticTypeCurrency: stringSemanticType{},
	FieldSemanticTypeCountry:  stringSemanticType{},
	FieldSemanticTypeMccCode:  stringSemanticType{},

	// Number family
	FieldSemanticTypeAmount:     numberSemanticType{},
	FieldSemanticTypePercentage: numberSemanticType{},
	FieldSemanticTypeQuantity:   numberSemanticType{},
}

// Use for input validation when creating/updating fields.
func (f FieldSemanticType) IsValid() bool {
	if f == FieldSemanticTypeUnset {
		return true
	}
	_, ok := fieldSemanticTypeRegistry[f]
	return ok
}

func (f FieldSemanticType) IsName() bool {
	return f == FieldSemanticTypeName ||
		f == FieldSemanticTypeFirstName ||
		f == FieldSemanticTypeMiddleName ||
		f == FieldSemanticTypeLastName
}

// ValidateField checks semantic type compatibility and cross-field constraints (primary ordering
// uniqueness). fields is the full list of fields for the table after the create/update is applied.
func ValidateField(field Field) error {
	if field.SemanticType == FieldSemanticTypeUnset {
		return nil
	}

	validator, ok := fieldSemanticTypeRegistry[field.SemanticType]
	if !ok {
		return errors.Wrap(BadParameterError, "unknown field semantic type")
	}
	if !slices.Contains(validator.AllowedDataTypes(), field.DataType) {
		return errors.Wrap(BadParameterError,
			fmt.Sprintf("field semantic type %q is not compatible with data type %s",
				field.SemanticType, field.DataType))
	}

	return nil
}
