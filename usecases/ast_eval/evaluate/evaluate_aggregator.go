package evaluate

import (
	"context"
	"fmt"
	"slices"

	"github.com/cockroachdb/errors"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/models/ast"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
)

type AggregatorEvaluator struct {
	OrganizationId             string
	DataModel                  models.DataModel
	ClientObject               models.ClientObject
	ExecutorFactory            executor_factory.ExecutorFactory
	IngestedDataReadRepository repositories.IngestedDataReadRepository
	ReturnFakeValue            bool
}

var ValidTypesForAggregator = map[ast.Aggregator][]models.DataType{
	ast.AGGREGATOR_AVG:            {models.Int, models.Float},
	ast.AGGREGATOR_COUNT:          {models.Bool, models.Int, models.Float, models.String, models.Timestamp},
	ast.AGGREGATOR_COUNT_DISTINCT: {models.Bool, models.Int, models.Float, models.String, models.Timestamp},
	ast.AGGREGATOR_MAX:            {models.Int, models.Float, models.Timestamp},
	ast.AGGREGATOR_MIN:            {models.Int, models.Float, models.Timestamp},
	ast.AGGREGATOR_SUM:            {models.Int, models.Float},
}

func (a AggregatorEvaluator) Evaluate(ctx context.Context, arguments ast.Arguments) (any, []error) {
	tableName, tableNameErr := AdaptNamedArgument(arguments.NamedArgs, "tableName", adaptArgumentToString)
	fieldName, fieldNameErr := AdaptNamedArgument(arguments.NamedArgs, "fieldName", adaptArgumentToString)
	_, labelErr := AdaptNamedArgument(arguments.NamedArgs, "label", adaptArgumentToString)
	aggregatorStr, aggregatorErr := AdaptNamedArgument(arguments.NamedArgs, "aggregator", adaptArgumentToString)
	filters, filtersErr := AdaptNamedArgument(arguments.NamedArgs, "filters",
		adaptArgumentToListOfThings[ast.Filter])

	errs := filterNilErrors(tableNameErr, fieldNameErr, labelErr, aggregatorErr, filtersErr)
	if len(errs) > 0 {
		return nil, errs
	}

	aggregator := ast.Aggregator(aggregatorStr)

	// Aggregator validation
	validTypes, isValid := ValidTypesForAggregator[aggregator]
	if !isValid {
		return MakeEvaluateError(errors.Join(
			errors.Wrap(ast.ErrRuntimeExpression,
				fmt.Sprintf("aggregator %s is not a valid aggregator in Evaluate aggregator", aggregator)),
			ast.NewNamedArgumentError("aggregator"),
		))
	}

	if tableName == "" && fieldName == "" {
		return MakeEvaluateError(errors.Join(
			ast.ErrAggregationFieldNotChosen,
			ast.NewNamedArgumentError("fieldName")))
	}
	fieldType, err := getFieldType(a.DataModel, tableName, fieldName)
	if err != nil {
		return MakeEvaluateError(errors.Join(
			errors.Wrap(err, fmt.Sprintf("field type for %s.%s not found in data model in Evaluate aggregator", tableName, fieldName)),
			ast.NewNamedArgumentError("fieldName"),
		))
	}
	isValidFieldType := slices.Contains(validTypes, fieldType)
	if !isValidFieldType {
		return MakeEvaluateError(errors.Join(
			ast.ErrAggregationFieldIncompatibleAggregator,
			ast.NewNamedArgumentError("fieldName"),
		))
	}

	// Filters validation
	var filtersWithType []models.FilterWithType
	if len(filters) > 0 {
		errs := make([]error, 0, len(filters))
		for idx, filter := range filters {
			if filter.TableName != tableName {
				errs = append(errs, errors.Join(
					ast.ErrFilterTableNotMatch,
					ast.NewNamedArgumentError(fmt.Sprintf("filters.%d.tableName", idx)),
				))
			}

			// At the first nil filter value found if we're not on an unary operator, stop and just return the default value for the aggregator
			if filter.Value == nil && !filter.Operator.IsUnary() {
				return a.defaultValueForAggregator(aggregator)
			}

			filterFieldType, err := getFieldType(a.DataModel, filter.TableName, filter.FieldName)
			if err != nil {
				errs = append(errs, errors.Join(
					errors.Wrap(err, fmt.Sprintf("field type for %s.%s not found in data model in Evaluate aggregator", filter.TableName, filter.FieldName)),
					ast.NewNamedArgumentError(fmt.Sprintf("filters.%d.fieldName", idx)),
				))
			}

			filtersWithType = append(filtersWithType, models.FilterWithType{
				Filter:    filter,
				FieldType: filterFieldType,
			})
		}

		if len(errs) > 0 {
			return nil, errs
		}
	}

	result, err := a.runQueryInRepository(ctx, tableName, fieldName, fieldType, aggregator, filtersWithType)
	if err != nil {
		return MakeEvaluateError(errors.Wrap(err, "Error running aggregation query in repository"))
	}

	if result == nil {
		return a.defaultValueForAggregator(aggregator)
	}

	return result, nil
}

func (a AggregatorEvaluator) runQueryInRepository(
	ctx context.Context,
	tableName string,
	fieldName string,
	fieldType models.DataType,
	aggregator ast.Aggregator,
	filters []models.FilterWithType,
) (any, error) {
	if a.ReturnFakeValue {
		return DryRunQueryAggregatedValue(a.DataModel, tableName, fieldName, aggregator)
	}

	db, err := a.ExecutorFactory.NewClientDbExecutor(ctx, a.OrganizationId)
	if err != nil {
		return nil, err
	}
	return a.IngestedDataReadRepository.QueryAggregatedValue(ctx, db, tableName,
		fieldName, fieldType, aggregator, filters)
}

func (a AggregatorEvaluator) defaultValueForAggregator(aggregator ast.Aggregator) (any, []error) {
	switch aggregator {
	case ast.AGGREGATOR_SUM:
		return 0.0, nil
	case ast.AGGREGATOR_COUNT, ast.AGGREGATOR_COUNT_DISTINCT:
		return 0, nil
	case ast.AGGREGATOR_AVG, ast.AGGREGATOR_MAX, ast.AGGREGATOR_MIN:
		return nil, nil
	default:
		return MakeEvaluateError(errors.Wrap(ast.ErrRuntimeExpression,
			fmt.Sprintf("aggregation %s not supported", aggregator)))
	}
}
