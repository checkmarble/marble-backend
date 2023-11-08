package payload_parser

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/checkmarble/marble-backend/models"
)

func TestParser_ValidatePayload(t *testing.T) {
	table := models.Table{
		Name:        "transactions",
		Description: "description",
		Fields: map[models.FieldName]models.Field{
			"optional": {
				DataType: models.String,
				Nullable: true,
			},
			"string": {
				DataType: models.String,
			},
			"integer": {
				DataType: models.Int,
			},
			"float": {
				DataType: models.Float,
			},
			"timestamp": {
				DataType: models.Timestamp,
			},
			"boolean": {
				DataType: models.Bool,
			},
		},
	}

	tests := []struct {
		name  string
		table models.Table
		input []byte
		want  map[models.FieldName]string
		err   error
	}{
		{
			name:  "nominal",
			table: table,
			input: []byte(`{
				"string": "string",
				"integer": 1000,
				"float": 10.10,
				"timestamp": "2023-10-19T17:33:22+02:00",
				"boolean": true
			}`),
		},
		{
			name:  "empty json",
			table: table,
			input: []byte(`{}`),
			want: map[models.FieldName]string{
				"string":    errIsNotNullable.Error(),
				"integer":   errIsNotNullable.Error(),
				"float":     errIsNotNullable.Error(),
				"timestamp": errIsNotNullable.Error(),
				"boolean":   errIsNotNullable.Error(),
			},
		},
		{
			name:  "bad json",
			input: []byte(`{bad}`),
			err:   errIsInvalidJSON,
		},
		{
			name:  "invalid fields",
			table: table,
			input: []byte(`{
				"string": 1000,
				"integer": "string",
				"float": "string",
				"timestamp": "not a timestamp",
				"boolean": "true"
			}`),
			want: map[models.FieldName]string{
				"string":    errIsInvalidString.Error(),
				"integer":   "is not a valid integer: expected an integer, got \"string\"",
				"float":     "is not a valid float: expected a float, got \"string\"",
				"timestamp": "is not a valid timestamp: expected format YYYY-MM-DDThh:mm:ss[+optional decimals]Z, got not a timestamp",
				"boolean":   "is not a valid boolean: expected a boolean, got \"true\"",
			},
		},
		{
			name: "invalid data type",
			table: models.Table{
				Name:        "transactions",
				Description: "description",
				Fields: map[models.FieldName]models.Field{
					"unknown": {
						DataType: models.UnknownDataType,
						Nullable: true,
					},
				},
			},
			input: []byte(`{"unknown": "unknown"}`),
			err:   errIsInvalidDataType,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewParser()

			errors, err := p.ValidatePayload(tt.table, tt.input)
			if tt.err != nil {
				assert.Error(t, err)
				assert.ErrorIs(t, err, tt.err)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tt.want, errors)
		})
	}
}
