package payload_parser

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/checkmarble/marble-backend/models"
)

func TestParser_ParsePayload(t *testing.T) {
	table := models.Table{
		Name:        "transactions",
		Description: "description",
		Fields: map[string]models.Field{
			"object_id": {
				DataType: models.String,
			},
			"updated_at": {
				DataType: models.Timestamp,
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
		name       string
		table      models.Table
		input      []byte
		wantErrors models.IngestionValidationErrors
		want       models.ClientObject
		err        error
	}{
		{
			name:  "nominal",
			table: table,
			input: []byte(`{
				"string": "string",
				"integer": 1000,
				"float": 10.10,
				"timestamp": "2023-10-19T17:33:22Z",
				"boolean": true,
				"object_id": "id",
				"updated_at": "2023-10-19 17:33:22"
			}`),
			want: models.ClientObject{
				TableName: "transactions",
				Data: map[string]any{
					"string":     "string",
					"integer":    int64(1000),
					"float":      10.10,
					"timestamp":  time.Date(2023, time.October, 19, 17, 33, 22, 0, time.UTC),
					"boolean":    true,
					"object_id":  "id",
					"updated_at": time.Date(2023, time.October, 19, 17, 33, 22, 0, time.UTC),
				},
			},
		},
		{
			name:  "empty json",
			table: table,
			input: []byte(`{}`),
			wantErrors: models.IngestionValidationErrors{"": models.IngestionValidationErrorsSingle{
				"string":     errIsNotNullable.Error(),
				"integer":    errIsNotNullable.Error(),
				"float":      errIsNotNullable.Error(),
				"timestamp":  errIsNotNullable.Error(),
				"boolean":    errIsNotNullable.Error(),
				"object_id":  errIsNotNullable.Error(),
				"updated_at": errIsNotNullable.Error(),
			}},
		},
		{
			name:  "missing fields other than object_id and updated_at",
			table: table,
			input: []byte(`{"object_id": "1", "updated_at": "2023-10-19 17:33:22"}`),
			wantErrors: models.IngestionValidationErrors{"1": models.IngestionValidationErrorsSingle{
				"string":    errIsNotNullable.Error(),
				"integer":   errIsNotNullable.Error(),
				"float":     errIsNotNullable.Error(),
				"timestamp": errIsNotNullable.Error(),
				"boolean":   errIsNotNullable.Error(),
			}},
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
			wantErrors: models.IngestionValidationErrors{"": models.IngestionValidationErrorsSingle{
				"string":     errIsInvalidString.Error(),
				"integer":    "is not a valid integer: expected an integer, got \"string\"",
				"float":      "is not a valid float: expected a float, got \"string\"",
				"timestamp":  "is not a valid timestamp: expected format \"YYYY-MM-DD hh:mm:ss[+optional decimals]\" or \"YYYY-MM-DDThh:mm:ss[+optional decimals]Z\", got \"not a timestamp\"",
				"boolean":    "is not a valid boolean: expected a boolean, got \"true\"",
				"object_id":  errIsNotNullable.Error(),
				"updated_at": errIsNotNullable.Error(),
			}},
		},
		{
			name: "invalid data type",
			table: models.Table{
				Name:        "transactions",
				Description: "description",
				Fields: map[string]models.Field{
					"unknown": {
						DataType: models.UnknownDataType,
						Nullable: true,
					},
				},
			},
			input: []byte(`{"unknown": "unknown"}`),
			err:   errIsInvalidDataType,
		},
		{
			name: "nullable fields without object_id and updated_at",
			table: models.Table{
				Name: "transactions",
				Fields: map[string]models.Field{
					"nullable": {
						DataType: models.String,
						Nullable: true,
					},
				},
			},
			input: []byte(`{}`),
			want: models.ClientObject{
				TableName: "transactions",
				Data: map[string]any{
					"nullable": nil,
				},
			},
			wantErrors: models.IngestionValidationErrors{"": models.IngestionValidationErrorsSingle{
				"object_id": errIsNotNullable.Error(),
			}},
		},
		{
			name: "nullable fields with object_id and updated_at",
			table: models.Table{
				Name: "transactions",
				Fields: map[string]models.Field{
					"nullable": {
						DataType: models.String,
						Nullable: true,
					},
					"object_id": {
						DataType: models.String,
						Nullable: false,
					},
					"updated_at": {
						DataType: models.Timestamp,
						Nullable: false,
					},
				},
			},
			input: []byte(`{"object_id": "id", "updated_at": "2023-10-19T00:00:00+03:00", "nullable": null}`),
			want: models.ClientObject{
				TableName: "transactions",
				Data: map[string]any{
					"nullable":  nil,
					"object_id": "id",
					// input is in UTC+3, but the output is in UTC
					"updated_at": time.Date(2023, time.October, 18, 21, 0, 0, 0, time.UTC),
				},
			},
		},
		{
			name: "nullable missing field with object_id and updated_at",
			table: models.Table{
				Name: "transactions",
				Fields: map[string]models.Field{
					"nullable": {
						DataType: models.String,
						Nullable: true,
					},
					"object_id": {
						DataType: models.String,
						Nullable: false,
					},
					"updated_at": {
						DataType: models.Timestamp,
						Nullable: false,
					},
				},
			},
			input: []byte(`{"object_id": "id", "updated_at": "2023-10-19T00:00:00+03:00"}`),
			want: models.ClientObject{
				TableName: "transactions",
				Data: map[string]any{
					"object_id": "id",
					// input is in UTC+3, but the output is in UTC
					"updated_at": time.Date(2023, time.October, 18, 21, 0, 0, 0, time.UTC),
				},
			},
		},
		{
			name: "nullable missing field with object_id and updated_at with timezone-less RFC3339 datetime",
			table: models.Table{
				Name: "transactions",
				Fields: map[string]models.Field{
					"nullable": {
						DataType: models.String,
						Nullable: true,
					},
					"object_id": {
						DataType: models.String,
						Nullable: false,
					},
					"updated_at": {
						DataType: models.Timestamp,
						Nullable: false,
					},
				},
			},
			input: []byte(`{"object_id": "id", "updated_at": "2023-10-19T00:00:00"}`),
			want: models.ClientObject{
				TableName: "transactions",
				Data: map[string]any{
					"object_id": "id",
					// input is in UTC+3, but the output is in UTC
					"updated_at": time.Date(2023, time.October, 19, 0, 0, 0, 0, time.UTC),
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewParser()

			out, err := p.ParsePayload(tt.table, tt.input)
			var validationErrors models.IngestionValidationErrors
			var otherErr error
			if !errors.As(err, &validationErrors) {
				otherErr = err
			}
			if otherErr != nil {
				assert.Error(t, tt.err)
				assert.ErrorIs(t, otherErr, tt.err, "error is the expected error")
			}
			if tt.err != nil {
				assert.ErrorIs(t, otherErr, tt.err, "expected this specific error")
			} else if len(tt.wantErrors) > 0 {
				assert.NoError(t, otherErr, "expected no global error")
				assert.Equal(t, tt.wantErrors, validationErrors, "expected those validation errors")
			} else if len(tt.want.Data) > 0 {
				assert.NoError(t, otherErr, "excepted no global error")
				assert.Empty(t, validationErrors, "expected no validation errors")
				assert.Equal(t, tt.want.Data, out.Data, "expected this client object")
			} else {
				t.Error("test case is not well defined")
			}
		})
	}
}
