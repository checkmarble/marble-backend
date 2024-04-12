package evaluate

import (
	"context"
	"testing"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/models/ast"

	"github.com/stretchr/testify/assert"
)

var dataModel = models.DataModel{
	Tables: map[models.TableName]models.Table{
		"table1": {
			Name: "table1",
			Fields: map[string]models.Field{
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

	result, errs := filter.Evaluate(context.TODO(), arguments)
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
	_, errs := filter.Evaluate(context.TODO(), arguments)
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
	_, errs := filter.Evaluate(context.TODO(), arguments)
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
	_, errs := filter.Evaluate(context.TODO(), arguments)
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
	_, errs := filter.Evaluate(context.TODO(), arguments)
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
	_, errs := filter.Evaluate(context.TODO(), arguments)
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
	_, errs := filter.Evaluate(context.TODO(), arguments)
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
	_, errs := filter.Evaluate(context.TODO(), arguments)
	assert.NotEmpty(t, errs)
}

var dataModelWithInt = models.DataModel{
	Tables: map[models.TableName]models.Table{
		"table1": {
			Name: "table1",
			Fields: map[string]models.Field{
				"field1": {
					DataType: models.Int,
					Nullable: false,
				},
			},
		},
	},
}
var filterWithInt = FilterEvaluator{DataModel: dataModelWithInt}

func TestFilter_value_float(t *testing.T) {
	arguments := ast.Arguments{
		NamedArgs: map[string]any{
			"tableName": "table1",
			"fieldName": "field1",
			"operator":  "=",
			"value":     10.1,
		},
	}

	expectedResult := ast.Filter{
		TableName: "table1",
		FieldName: "field1",
		Operator:  ast.FILTER_EQUAL,
		Value:     10.1,
	}
	result, errs := filterWithInt.Evaluate(context.TODO(), arguments)
	assert.Empty(t, errs)

	assert.ObjectsAreEqualValues(expectedResult, result)
}

var dataModelWithString = models.DataModel{
	Tables: map[models.TableName]models.Table{
		"table1": {
			Name: "table1",
			Fields: map[string]models.Field{
				"field1": {
					DataType: models.String,
					Nullable: false,
				},
			},
		},
	},
}
var filterWithString = FilterEvaluator{DataModel: dataModelWithString}

func TestFilter_is_in_list(t *testing.T) {
	arguments := ast.Arguments{
		NamedArgs: map[string]any{
			"tableName": "table1",
			"fieldName": "field1",
			"operator":  "IsInList",
			"value":     []string{"a", "b"},
		},
	}

	expectedResult := ast.Filter{
		TableName: "table1",
		FieldName: "field1",
		Operator:  ast.FILTER_IS_IN_LIST,
		Value:     []string{"a", "b"},
	}
	result, errs := filterWithString.Evaluate(context.TODO(), arguments)
	assert.Empty(t, errs)

	assert.ObjectsAreEqualValues(expectedResult, result)
}

func TestFilter_is_not_in_list(t *testing.T) {
	arguments := ast.Arguments{
		NamedArgs: map[string]any{
			"tableName": "table1",
			"fieldName": "field1",
			"operator":  "IsNotInList",
			"value":     []string{"a", "b"},
		},
	}

	expectedResult := ast.Filter{
		TableName: "table1",
		FieldName: "field1",
		// Operator:  ast.FILTER_IS_NOT_IN_LIST,
		Value: []string{"a", "b"},
	}
	result, errs := filterWithString.Evaluate(context.TODO(), arguments)
	assert.Empty(t, errs)

	assert.ObjectsAreEqualValues(expectedResult, result)
}

func TestFilter_is_in_list_invalid_value_type(t *testing.T) {
	arguments := ast.Arguments{
		NamedArgs: map[string]any{
			"tableName": "table1",
			"fieldName": "field1",
			"operator":  "IsInList",
			"value":     []int{1, 2},
		},
	}

	_, errs := filterWithString.Evaluate(context.TODO(), arguments)
	assert.NotEmpty(t, errs)
}

func TestFilter_is_in_list_invalid_field_type(t *testing.T) {
	arguments := ast.Arguments{
		NamedArgs: map[string]any{
			"tableName": "table1",
			"fieldName": "field1",
			"operator":  "IsInList",
			"value":     []string{"a", "b"},
		},
	}

	_, errs := filterWithInt.Evaluate(context.TODO(), arguments)
	assert.NotEmpty(t, errs)
}
