package evaluate

import (
	"fmt"
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
}

func (f FilterEvaluator) Evaluate(arguments ast.Arguments) (any, []error) {
	tableNameStr, tableNameErr := AdaptNamedArgument(arguments.NamedArgs, "tableName", adaptArgumentToString)
	fieldNameStr, fieldNameErr := AdaptNamedArgument(arguments.NamedArgs, "fieldName", adaptArgumentToString)
	operatorStr, operatorErr := AdaptNamedArgument(arguments.NamedArgs, "operator", adaptArgumentToString)

	errs := filterNilErrors(tableNameErr, fieldNameErr, operatorErr)
	if len(errs) > 0 {
		return nil, errs
	}

	fieldType, err := getFieldType(f.DataModel, models.TableName(tableNameStr), models.FieldName(fieldNameStr))
	if err != nil {
		return MakeEvaluateError(fmt.Errorf("field type for %s.%s not found in data model %w %w", tableNameStr, fieldNameStr, err, models.ErrRuntimeExpression))
	}

	// Operator validation
	operator := ast.FilterOperator(operatorStr)
	validTypes, isValid := ValidTypeForFilterOperators[operator]
	if !isValid {
		return MakeEvaluateError(fmt.Errorf("operator is not a valid operator %w", models.ErrRuntimeExpression))
	}

	isValidFieldType := slices.Contains(validTypes, fieldType)
	if !isValidFieldType {
		return MakeEvaluateError(fmt.Errorf("field type %s is not valid for operator %s %w", fieldType.String(), operator, models.ErrRuntimeExpression))
	}

	// Value validation
	value := arguments.NamedArgs["value"]
	promotedValue, err := promoteArgumentToDataType(value, fieldType)
	if err != nil {
		return MakeEvaluateError(err)
	}

	returnValue := ast.Filter{
		TableName: tableNameStr,
		FieldName: fieldNameStr,
		Operator:  operator,
		Value:     promotedValue,
	}
	return returnValue, nil
}
