package evaluate

import (
	"context"
	"fmt"
	"slices"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/models/ast"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/transaction"
)

type AggregatorEvaluator struct {
	OrganizationId             string
	DataModel                  models.DataModel
	Payload                    models.PayloadReader
	OrgTransactionFactory      transaction.Factory
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
	filters, filtersErr := AdaptNamedArgument(arguments.NamedArgs, "filters", adaptArgumentToListOfThings[ast.Filter])

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
		return MakeEvaluateError(fmt.Errorf("%s is not a valid aggregator %w %w", aggregator, models.ErrRuntimeExpression, ast.NewNamedArgumentError("aggregator")))
	}

	fieldType, err := getFieldType(a.DataModel, tableName, fieldName)
	if err != nil {
		return MakeEvaluateError(fmt.Errorf("field type for %s.%s not found in data model %w %w", tableName, fieldName, err, ast.NewNamedArgumentError("fieldName")))
	}
	isValidFieldType := slices.Contains(validTypes, fieldType)
	if !isValidFieldType {
		return MakeEvaluateError(fmt.Errorf("field type %s is not valid for aggregator %s %w %w", fieldType, aggregator, models.ErrRuntimeExpression, ast.NewNamedArgumentError("fieldName")))
	}

	// Filters validation
	if len(filters) > 0 {
		for _, filter := range filters {
			if filter.TableName != tableNameStr {
				return MakeEvaluateError(fmt.Errorf("filters must be applied on the same table %w %w", models.ErrRuntimeExpression, ast.NewNamedArgumentError("filters")))
			}
		}
	}

	result, err := a.runQueryInRepository(ctx, tableName, fieldName, aggregator, filters)
	if err != nil {
		return MakeEvaluateError(err)
	}

	if result == nil {
		return a.defaultValueForAggregator(aggregator)
	}

	return result, nil
}

func (a AggregatorEvaluator) runQueryInRepository(ctx context.Context, tableName models.TableName, fieldName models.FieldName, aggregator ast.Aggregator, filters []ast.Filter) (any, error) {
	if a.ReturnFakeValue {
		return DryRunQueryAggregatedValue(a.DataModel, tableName, fieldName, aggregator)
	}

	return transaction.InOrganizationSchema(
		ctx,
		a.OrgTransactionFactory,
		a.OrganizationId,
		func(tx repositories.Transaction) (any, error) {
			result, err := a.IngestedDataReadRepository.QueryAggregatedValue(ctx, tx, tableName, fieldName, aggregator, filters)
			if err != nil {
				return nil, err
			}
			return result, nil
		},
	)
}

func (a AggregatorEvaluator) defaultValueForAggregator(aggregator ast.Aggregator) (any, []error) {
	switch aggregator {
	case ast.AGGREGATOR_SUM:
		return 0.0, nil
	case ast.AGGREGATOR_COUNT, ast.AGGREGATOR_COUNT_DISTINCT:
		return 0, nil
	case ast.AGGREGATOR_AVG, ast.AGGREGATOR_MAX, ast.AGGREGATOR_MIN:
		return MakeEvaluateError(fmt.Errorf("aggregation %s returned null %w", aggregator, models.NullFieldReadError))
	default:
		return MakeEvaluateError(fmt.Errorf("aggregation %s not supported %w", aggregator, models.ErrRuntimeExpression))
	}
}
