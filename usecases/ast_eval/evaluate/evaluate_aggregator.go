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
	tableNameStr, tableNameErr := AdaptNamedArgument(arguments.NamedArgs, "tableName", adaptArgumentToString)
	fieldNameStr, fieldNameErr := AdaptNamedArgument(arguments.NamedArgs, "fieldName", adaptArgumentToString)
	_, labelErr := AdaptNamedArgument(arguments.NamedArgs, "label", adaptArgumentToString)
	aggregatorStr, aggregatorErr := AdaptNamedArgument(arguments.NamedArgs, "aggregator", adaptArgumentToString)
	filters, filtersErr := AdaptNamedArgument(arguments.NamedArgs, "filters",
		adaptArgumentToListOfThings[ast.Filter])

	errs := filterNilErrors(tableNameErr, fieldNameErr, labelErr, aggregatorErr, filtersErr)
	if len(errs) > 0 {
		return nil, errs
	}

	tableName := models.TableName(tableNameStr)
	fieldName := models.FieldName(fieldNameStr)
	aggregator := ast.Aggregator(aggregatorStr)

	// Aggregator validation
	validTypes, isValid := ValidTypesForAggregator[aggregator]
	if !isValid {
		return MakeEvaluateError(errors.Join(
			errors.Wrap(models.ErrRuntimeExpression,
				fmt.Sprintf("aggregator %s is not a valid aggregator in Evaluate aggregator", aggregator)),
			ast.NewNamedArgumentError("aggregator"),
		))
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
			errors.Wrap(models.ErrRuntimeExpression,
				fmt.Sprintf("field type %s is not valid for aggregator %s in Evaluate aggregator", fieldType.String(), aggregator)),
			ast.NewNamedArgumentError("fieldName"),
		))
	}

	// Filters validation
	if len(filters) > 0 {
		for _, filter := range filters {
			if filter.TableName != tableNameStr {
				return MakeEvaluateError(errors.Join(
					errors.Wrap(models.ErrRuntimeExpression,
						"filters must be applied on the same table"),
					ast.NewNamedArgumentError("filters"),
				))
			}
		}
	}

	result, err := a.runQueryInRepository(ctx, tableName, fieldName, aggregator, filters)
	if err != nil {
		return MakeEvaluateError(errors.Wrap(err, "Error running aggregation query in repository"))
	}

	if result == nil {
		return a.defaultValueForAggregator(aggregator)
	}

	return result, nil
}

func (a AggregatorEvaluator) runQueryInRepository(ctx context.Context, tableName models.TableName,
	fieldName models.FieldName, aggregator ast.Aggregator, filters []ast.Filter,
) (any, error) {
	if a.ReturnFakeValue {
		return DryRunQueryAggregatedValue(a.DataModel, tableName, fieldName, aggregator)
	}

	db, err := a.ExecutorFactory.NewClientDbExecutor(ctx, a.OrganizationId)
	if err != nil {
		return nil, err
	}
	return a.IngestedDataReadRepository.QueryAggregatedValue(ctx, db, tableName, fieldName, aggregator, filters)
}

func (a AggregatorEvaluator) defaultValueForAggregator(aggregator ast.Aggregator) (any, []error) {
	switch aggregator {
	case ast.AGGREGATOR_SUM:
		return 0.0, nil
	case ast.AGGREGATOR_COUNT, ast.AGGREGATOR_COUNT_DISTINCT:
		return 0, nil
	case ast.AGGREGATOR_AVG, ast.AGGREGATOR_MAX, ast.AGGREGATOR_MIN:
		return MakeEvaluateError(errors.Wrap(models.ErrNullFieldRead,
			fmt.Sprintf("aggregation %s returned null", aggregator)))
	default:
		return MakeEvaluateError(errors.Wrap(models.ErrRuntimeExpression,
			fmt.Sprintf("aggregation %s not supported", aggregator)))
	}
}
