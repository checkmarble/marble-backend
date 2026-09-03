package models

import (
	"encoding/json"
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

type stringOrNumberSemanticType struct{}

func (stringOrNumberSemanticType) AllowedDataTypes() []DataType { return []DataType{String, Int, Float} }

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
	FieldSemanticTypeEnum:     stringOrNumberSemanticType{},
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

// FieldSemanticSubType refines what a field means beyond its semantic type. Unlike the semantic
// type, it is not a column of its own: it lives in the field's metadata blob, under
// `semanticSubType`, and is written there by whoever edits the data model.
type FieldSemanticSubType string

const (
	FieldSemanticSubTypeUnset FieldSemanticSubType = ""

	// FieldSemanticSubTypeCaption marks the field holding what a record of its table is called.
	// It is what a graph node, for one, is labelled with.
	FieldSemanticSubTypeCaption FieldSemanticSubType = "caption"
)

// fieldMetadata is the part of a field's metadata blob this package reads. The blob holds whatever
// the data model editor put there, so it is decoded key by key rather than replaced: anything not
// named here is left alone. Its keys are camelCase, unlike the snake_case of the API's own
// payloads, because they are the editor's and not ours to rename.
type fieldMetadata struct {
	SemanticSubType FieldSemanticSubType `json:"semanticSubType"` //nolint:tagliatelle
}

// SemanticSubType returns the sub-type a field declares in its metadata. A field with no metadata,
// none declared, or metadata that cannot be read declares none: the blob is free-form, so failing
// to decode it says nothing more than that this field is not the one being looked for.
func (f Field) SemanticSubType() FieldSemanticSubType {
	if len(f.Metadata) == 0 {
		return FieldSemanticSubTypeUnset
	}

	var metadata fieldMetadata

	if err := json.Unmarshal(f.Metadata, &metadata); err != nil {
		return FieldSemanticSubTypeUnset
	}

	return metadata.SemanticSubType
}

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
