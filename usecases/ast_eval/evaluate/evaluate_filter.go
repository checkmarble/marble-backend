package evaluate

import (
	"context"
	"fmt"
	"reflect"
	"slices"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/models/ast"
)

type FilterEvaluator struct {
	DataModel models.DataModel
}

var ValidTypeForFilterOperators = map[ast.FilterOperator][]models.DataType{
	ast.FILTER_EQUAL:            {models.Bool, models.Int, models.Float, models.String, models.Timestamp},
	ast.FILTER_NOT_EQUAL:        {models.Bool, models.Int, models.Float, models.String, models.Timestamp},
	ast.FILTER_GREATER:          {models.Int, models.Float, models.String, models.Timestamp},
	ast.FILTER_GREATER_OR_EQUAL: {models.Int, models.Float, models.String, models.Timestamp},
	ast.FILTER_LESSER:           {models.Int, models.Float, models.String, models.Timestamp},
	ast.FILTER_LESSER_OR_EQUAL:  {models.Int, models.Float, models.String, models.Timestamp},
	ast.FILTER_IS_IN_LIST:       {models.String},
	ast.FILTER_IS_NOT_IN_LIST:   {models.String},
}

func (f FilterEvaluator) Evaluate(ctx context.Context, arguments ast.Arguments) (any, []error) {
	tableNameStr, tableNameErr := AdaptNamedArgument(arguments.NamedArgs, "tableName", adaptArgumentToString)
	fieldNameStr, fieldNameErr := AdaptNamedArgument(arguments.NamedArgs, "fieldName", adaptArgumentToString)
	operatorStr, operatorErr := AdaptNamedArgument(arguments.NamedArgs, "operator", adaptArgumentToString)

	errs := filterNilErrors(tableNameErr, fieldNameErr, operatorErr)
	if len(errs) > 0 {
		return nil, errs
	}

	fieldType, err := getFieldType(f.DataModel, models.TableName(tableNameStr), models.FieldName(fieldNameStr))
	if err != nil {
		return MakeEvaluateError(fmt.Errorf("field type for %s.%s not found in data model %w %w", tableNameStr, fieldNameStr, err, ast.NewNamedArgumentError("fieldName")))
	}

	// Operator validation
	operator := ast.FilterOperator(operatorStr)
	validTypes, isValid := ValidTypeForFilterOperators[operator]
	if !isValid {
		return MakeEvaluateError(fmt.Errorf("operator is not a valid operator %w %w", models.ErrRuntimeExpression, ast.NewNamedArgumentError("operator")))
	}

	isValidFieldType := slices.Contains(validTypes, fieldType)
	if !isValidFieldType {
		return MakeEvaluateError(fmt.Errorf("field type %s is not valid for operator %s %w %w", fieldType.String(), operator, ast.ErrArgumentInvalidType, ast.NewNamedArgumentError("fieldName")))
	}

	// Value validation
	value := arguments.NamedArgs["value"]
	var promotedValue any
	// When value is a float, it cannot be cast to int but SQL can handle the comparision, so no casting is required
	if fieldType == models.Int && reflect.TypeOf(value) == reflect.TypeOf(float64(0)) {
		promotedValue = value
	} else {
		if operator == ast.FILTER_IS_IN_LIST || operator == ast.FILTER_IS_NOT_IN_LIST {
			promotedValue, err = adaptArgumentToListOfStrings(value)
		} else {
			promotedValue, err = promoteArgumentToDataType(value, fieldType)
		}
		if err != nil {
			return MakeEvaluateError(fmt.Errorf("value is not compatible with selected field %w %w: %w", ast.ErrArgumentInvalidType, ast.NewNamedArgumentError("value"), err))
		}
	}

	returnValue := ast.Filter{
		TableName: tableNameStr,
		FieldName: fieldNameStr,
		Operator:  operator,
		Value:     promotedValue,
	}
	return returnValue, nil
}
