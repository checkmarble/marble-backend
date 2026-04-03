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

	// Enum family
	FieldSemanticTypeEnum     FieldSemanticType = "enum"
	FieldSemanticTypeCurrency FieldSemanticType = "currency"
	FieldSemanticTypeCountry  FieldSemanticType = "country"

	// Address family
	FieldSemanticTypeAddress FieldSemanticType = "address"

	// Unique ID family
	FieldSemanticTypeId                 FieldSemanticType = "id"
	FieldSemanticTypeRegistrationNumber FieldSemanticType = "registration_number"
	FieldSemanticTypeTaxId              FieldSemanticType = "tax_id"
	FieldSemanticTypeAccountNumber      FieldSemanticType = "account_number"
	FieldSemanticTypeIban               FieldSemanticType = "iban"
	FieldSemanticTypeBic                FieldSemanticType = "bic"
	FieldSemanticTypeForeignKey         FieldSemanticType = "foreign_key"

	// URL family
	FieldSemanticTypeUrl         FieldSemanticType = "url"
	FieldSemanticTypeEmail       FieldSemanticType = "email"
	FieldSemanticTypePhoneNumber FieldSemanticType = "phone_number"

	// Time family
	FieldSemanticTypeDateOfBirth    FieldSemanticType = "date_of_birth"
	FieldSemanticTypeLastUpdate     FieldSemanticType = "last_update"
	FieldSemanticTypeCreationDate   FieldSemanticType = "creation_date"
	FieldSemanticTypeDeletionDate   FieldSemanticType = "deletion_date"
	FieldSemanticTypeInitiationDate FieldSemanticType = "initiation_date"
	FieldSemanticTypeValidationDate FieldSemanticType = "validation_date"

	// Number family
	FieldSemanticTypeMonetaryAmount FieldSemanticType = "monetary_amount"
	FieldSemanticTypePercentage     FieldSemanticType = "percentage"
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

	// Enum family
	FieldSemanticTypeEnum:     stringSemanticType{},
	FieldSemanticTypeCurrency: stringSemanticType{},
	FieldSemanticTypeCountry:  stringSemanticType{},

	// Address family
	FieldSemanticTypeAddress: stringSemanticType{},

	// Unique ID family
	FieldSemanticTypeId:                 stringSemanticType{},
	FieldSemanticTypeRegistrationNumber: stringSemanticType{},
	FieldSemanticTypeTaxId:              stringSemanticType{},
	FieldSemanticTypeAccountNumber:      stringSemanticType{},
	FieldSemanticTypeIban:               stringSemanticType{},
	FieldSemanticTypeBic:                stringSemanticType{},
	FieldSemanticTypeForeignKey:         stringSemanticType{},

	// URL family
	FieldSemanticTypeUrl:         stringSemanticType{},
	FieldSemanticTypeEmail:       stringSemanticType{},
	FieldSemanticTypePhoneNumber: stringSemanticType{},

	// Time family
	FieldSemanticTypeDateOfBirth:    timestampSemanticType{},
	FieldSemanticTypeLastUpdate:     timestampSemanticType{},
	FieldSemanticTypeCreationDate:   timestampSemanticType{},
	FieldSemanticTypeDeletionDate:   timestampSemanticType{},
	FieldSemanticTypeInitiationDate: timestampSemanticType{},
	FieldSemanticTypeValidationDate: timestampSemanticType{},

	// Number family
	FieldSemanticTypeMonetaryAmount: numberSemanticType{},
	FieldSemanticTypePercentage:     numberSemanticType{},
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
