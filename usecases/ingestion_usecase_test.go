package usecases

import (
	"testing"

	"github.com/checkmarble/marble-backend/models"
)

func TestParseStringValuesToMap(t *testing.T) {
	table := models.Table{
		Name: "transactions",
		Fields: map[models.FieldName]models.Field{
			"object_id": {
				DataType: models.String, Nullable: false,
			},
			"updated_at": {DataType: models.Timestamp, Nullable: false},
			"value":      {DataType: models.Float, Nullable: true},
			"status":     {DataType: models.String, Nullable: true},
		},
		LinksToSingle: nil,
	}

	type testCase struct {
		name    string
		columns []string
		values  []string
	}

	OKcases := []testCase{
		{
			name:    "valid case with all fields present",
			columns: []string{"object_id", "updated_at", "value", "status"},
			values:  []string{"1", "2020-01-01T00:00:00Z", "1.0", "OK"},
		},
		{
			name:    "valid case with empty status and null value",
			columns: []string{"object_id", "updated_at", "value", "status"},
			values:  []string{"1", "2020-01-01T00:00:00Z", "", ""},
		},
	}

	for _, c := range OKcases {
		_, err := parseStringValuesToMap(c.columns, c.values, table)
		if err != nil {
			t.Errorf("Error parsing string values to map: %v", err)
		}
	}

	ErrCases := []testCase{
		{
			name:    "error case with missing object_id",
			columns: []string{"object_id", "updated_at", "value", "status"},
			values:  []string{"", "2020-01-01T00:00:00Z", "", ""},
		},
		{
			name:    "error case with missing updated_at",
			columns: []string{"object_id", "updated_at", "value", "status"},
			values:  []string{"1234", "", "", ""},
		},
		{
			name:    "error case with bad format updated_at (missing T & Z)",
			columns: []string{"object_id", "updated_at", "value", "status"},
			values:  []string{"1234", "2023-01-01 00:00:00", "", ""},
		},
		{
			name:    "error case with bad format value",
			columns: []string{"object_id", "updated_at", "value", "status"},
			values:  []string{"1234", "2023-01-01T00:00:00Z", "This is not a number", ""},
		},
	}
	for _, c := range ErrCases {
		_, err := parseStringValuesToMap(c.columns, c.values, table)
		if err == nil {
			t.Errorf("Expected error parsing string values to map: %v", err)
		}
	}
}
