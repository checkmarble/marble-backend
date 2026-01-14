package continuous_screening

import (
	"testing"

	"github.com/checkmarble/marble-backend/models"
	"github.com/stretchr/testify/assert"
)

func TestCaseNameBuilderFromIngestedObject(t *testing.T) {
	tests := []struct {
		name           string
		ingestedObject models.DataModelObject
		mapping        models.ContinuousScreeningDataModelMapping
		expected       string
		expectError    bool
	}{
		{
			name: "Name property found",
			ingestedObject: models.DataModelObject{
				Data: map[string]any{
					"object_id":  "obj123",
					"name_field": "John Doe Corp",
				},
			},
			mapping: models.ContinuousScreeningDataModelMapping{
				Properties: map[string]string{
					"name_field": "name",
				},
			},
			expected: "John Doe Corp",
		},
		{
			name: "FirstName and LastName combined",
			ingestedObject: models.DataModelObject{
				Data: map[string]any{
					"object_id":        "obj123",
					"first_name_field": "John",
					"last_name_field":  "Doe",
				},
			},
			mapping: models.ContinuousScreeningDataModelMapping{
				Properties: map[string]string{
					"first_name_field": "firstName",
					"last_name_field":  "lastName",
				},
			},
			expected: "Doe John",
		},
		{
			name: "LastName only",
			ingestedObject: models.DataModelObject{
				Data: map[string]any{
					"object_id":       "obj123",
					"last_name_field": "Doe",
				},
			},
			mapping: models.ContinuousScreeningDataModelMapping{
				Properties: map[string]string{
					"last_name_field": "lastName",
				},
			},
			expected: "Doe",
		},
		{
			name: "FirstName only",
			ingestedObject: models.DataModelObject{
				Data: map[string]any{
					"object_id":        "obj123",
					"first_name_field": "John",
				},
			},
			mapping: models.ContinuousScreeningDataModelMapping{
				Properties: map[string]string{
					"first_name_field": "firstName",
				},
			},
			expected: "John",
		},
		{
			name: "RegistrationNumber found",
			ingestedObject: models.DataModelObject{
				Data: map[string]any{
					"object_id": "obj123",
					"reg_field": "REG123456",
				},
			},
			mapping: models.ContinuousScreeningDataModelMapping{
				Properties: map[string]string{
					"reg_field": "registrationNumber",
				},
			},
			expected: "REG123456",
		},
		{
			name: "ImoNumber found",
			ingestedObject: models.DataModelObject{
				Data: map[string]any{
					"object_id": "obj123",
					"imo_field": "IMO987654",
				},
			},
			mapping: models.ContinuousScreeningDataModelMapping{
				Properties: map[string]string{
					"imo_field": "imoNumber",
				},
			},
			expected: "IMO987654",
		},
		{
			name: "Fallback to objectId",
			ingestedObject: models.DataModelObject{
				Data: map[string]any{
					"object_id": "obj123",
				},
			},
			mapping: models.ContinuousScreeningDataModelMapping{
				Properties: map[string]string{},
			},
			expected: "obj123",
		},
		{
			name: "Priority: Name over FirstName/LastName",
			ingestedObject: models.DataModelObject{
				Data: map[string]any{
					"object_id":        "obj123",
					"name_field":       "Company Name",
					"first_name_field": "John",
					"last_name_field":  "Doe",
				},
			},
			mapping: models.ContinuousScreeningDataModelMapping{
				Properties: map[string]string{
					"name_field":       "name",
					"first_name_field": "firstName",
					"last_name_field":  "lastName",
				},
			},
			expected: "Company Name",
		},
		{
			name: "Priority: FirstName/LastName over RegistrationNumber",
			ingestedObject: models.DataModelObject{
				Data: map[string]any{
					"object_id":        "obj123",
					"first_name_field": "John",
					"last_name_field":  "Doe",
					"reg_field":        "REG123",
				},
			},
			mapping: models.ContinuousScreeningDataModelMapping{
				Properties: map[string]string{
					"first_name_field": "firstName",
					"last_name_field":  "lastName",
					"reg_field":        "registrationNumber",
				},
			},
			expected: "Doe John",
		},
		{
			name: "Empty strings are ignored",
			ingestedObject: models.DataModelObject{
				Data: map[string]any{
					"object_id":        "obj123",
					"name_field":       "",
					"first_name_field": "   ",
					"last_name_field":  "",
				},
			},
			mapping: models.ContinuousScreeningDataModelMapping{
				Properties: map[string]string{
					"name_field":       "name",
					"first_name_field": "firstName",
					"last_name_field":  "lastName",
				},
			},
			expected: "obj123",
		},
		{
			name: "Numeric values are converted to strings",
			ingestedObject: models.DataModelObject{
				Data: map[string]any{
					"object_id": "obj123",
					"reg_field": 12345,
				},
			},
			mapping: models.ContinuousScreeningDataModelMapping{
				Properties: map[string]string{
					"reg_field": "registrationNumber",
				},
			},
			expected: "12345",
		},
		{
			name: "Complete priority order test",
			ingestedObject: models.DataModelObject{
				Data: map[string]any{
					"object_id":        "fallback-id",
					"name_field":       "Company Name Inc",
					"first_name_field": "John",
					"last_name_field":  "Doe",
					"reg_field":        "REG123456",
					"imo_field":        "IMO987654",
				},
			},
			mapping: models.ContinuousScreeningDataModelMapping{
				Properties: map[string]string{
					"name_field":       "name",
					"first_name_field": "firstName",
					"last_name_field":  "lastName",
					"reg_field":        "registrationNumber",
					"imo_field":        "imoNumber",
				},
			},
			expected: "Company Name Inc", // Name should take priority over all others
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := caseNameBuilderFromIngestedObject(tt.ingestedObject, tt.mapping)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}
