package evaluate

import (
	"testing"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/models/ast"

	"github.com/stretchr/testify/assert"
)

var dataModel = models.DataModel{
	Tables: map[models.TableName]models.Table{
		"table1": {
			Name: "table1",
			Fields: map[models.FieldName]models.Field{
				"field1": {
					DataType: models.Bool,
					Nullable: false,
				},
			},
		},
	},
}
var filter = FilterEvaluator{DataModel: dataModel}

func TestFilter(t *testing.T) {
	arguments := ast.Arguments{
		NamedArgs: map[string]any{
			"tableName": "table1",
			"fieldName": "field1",
			"operator":  "=",
			"value":     true,
		},
	}
	expectedResult := ast.Filter{
		TableName: "table1",
		FieldName: "field1",
		Operator:  ast.FILTER_EQUAL,
		Value:     1,
	}

	result, errs := filter.Evaluate(arguments)
	assert.Empty(t, errs)
	assert.ObjectsAreEqualValues(expectedResult, result)
}

func TestFilter_tableName_not_string(t *testing.T) {
	arguments := ast.Arguments{
		NamedArgs: map[string]any{
			"tableName": 0,
			"fieldName": "field1",
			"operator":  "=",
			"value":     1,
		},
	}
	_, errs := filter.Evaluate(arguments)
	assert.NotEmpty(t, errs)
}

func TestFilter_fieldName_not_string(t *testing.T) {
	arguments := ast.Arguments{
		NamedArgs: map[string]any{
			"tableName": "table1",
			"fieldName": 0,
			"operator":  "=",
			"value":     1,
		},
	}
	_, errs := filter.Evaluate(arguments)
	assert.NotEmpty(t, errs)
}

func TestFilter_field_unknown(t *testing.T) {
	arguments := ast.Arguments{
		NamedArgs: map[string]any{
			"tableName": "table1",
			"fieldName": "unknown_field",
			"operator":  "=",
			"value":     1,
		},
	}
	_, errs := filter.Evaluate(arguments)
	assert.NotEmpty(t, errs)
}

func TestFilter_operator_invalid(t *testing.T) {
	arguments := ast.Arguments{
		NamedArgs: map[string]any{
			"tableName": "table1",
			"fieldName": "field1",
			"operator":  0,
			"value":     1,
		},
	}
	_, errs := filter.Evaluate(arguments)
	assert.NotEmpty(t, errs)
}

func TestFilter_operator_unknown(t *testing.T) {
	arguments := ast.Arguments{
		NamedArgs: map[string]any{
			"tableName": "table1",
			"fieldName": "field1",
			"operator":  "invalid_operator",
			"value":     1,
		},
	}
	_, errs := filter.Evaluate(arguments)
	assert.NotEmpty(t, errs)
}

func TestFilter_fieldType_incompatible(t *testing.T) {
	arguments := ast.Arguments{
		NamedArgs: map[string]any{
			"tableName": "table1",
			"fieldName": "field1",
			"operator":  ">",
			"value":     1,
		},
	}
	_, errs := filter.Evaluate(arguments)
	assert.NotEmpty(t, errs)
}

func TestFilter_value_incompatible(t *testing.T) {
	arguments := ast.Arguments{
		NamedArgs: map[string]any{
			"tableName": "table1",
			"fieldName": "field1",
			"operator":  "=",
			"value":     "incompatible_value",
		},
	}
	_, errs := filter.Evaluate(arguments)
	assert.NotEmpty(t, errs)
}
