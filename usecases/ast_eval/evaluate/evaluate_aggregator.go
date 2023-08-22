package evaluate

import (
	"fmt"
	"marble/marble-backend/models"
	"marble/marble-backend/models/ast"
	"marble/marble-backend/repositories"
	"marble/marble-backend/usecases/org_transaction"
	"slices"
)

type AggregatorEvaluator struct {
	OrganizationId             string
	DataModel                  models.DataModel
	Payload                    models.PayloadReader
	OrgTransactionFactory      org_transaction.Factory
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

func (a AggregatorEvaluator) Evaluate(arguments ast.Arguments) (any, []error) {
	tableNameStr, err := adaptArgumentToString(arguments.NamedArgs["tableName"])
	if err != nil {
		return MakeEvaluateError(err)
	}
	tableName := models.TableName(tableNameStr)

	fieldNameStr, err := adaptArgumentToString(arguments.NamedArgs["fieldName"])
	if err != nil {
		return MakeEvaluateError(err)
	}
	fieldName := models.FieldName(fieldNameStr)

	// Aggregator validation
	aggregatorStr, err := adaptArgumentToString(arguments.NamedArgs["aggregator"])
	if err != nil {
		return MakeEvaluateError(err)
	}
	aggregator := ast.Aggregator(aggregatorStr)
	validTypes, isValid := ValidTypesForAggregator[aggregator]
	if !isValid {
		return MakeEvaluateError(fmt.Errorf("%s is not a valid aggregator %w", aggregator, models.ErrRuntimeExpression))
	}

	fieldType, err := getFieldType(a.DataModel, tableName, fieldName)
	if err != nil {
		return MakeEvaluateError(fmt.Errorf("field type for %s.%s not found in data model %w %w", tableName, fieldName, err, models.ErrRuntimeExpression))
	}
	isValidFieldType := slices.Contains(validTypes, fieldType)
	if !isValidFieldType {
		return MakeEvaluateError(fmt.Errorf("field type %s is not valid for aggregator %s %w", fieldType, aggregator, models.ErrRuntimeExpression))
	}

	// Filters validation
	filters, err := adaptArgumentToListOfThings[ast.Filter](arguments.NamedArgs["filters"])
	if err != nil {
		return MakeEvaluateError(err)
	}
	if len(filters) > 0 {
		for _, filter := range filters {
			if filter.TableName != tableNameStr {
				return MakeEvaluateError(fmt.Errorf("filters must be applied on the same table %w", models.ErrRuntimeExpression))
			}
		}
	}

	result, err := a.runQueryInRepository(tableName, fieldName, aggregator, filters)
	if err != nil {
		return MakeEvaluateError(err)
	}

	if result == nil {
		return a.defaultValueForAggregator(aggregator)
	}

	return result, nil
}

func (a AggregatorEvaluator) runQueryInRepository(tableName models.TableName, fieldName models.FieldName, aggregator ast.Aggregator, filters []ast.Filter) (any, error) {
	if a.ReturnFakeValue {
		return DryRunQueryAggregatedValue(a.DataModel, tableName, fieldName, aggregator)
	}

	return org_transaction.InOrganizationSchema(
		a.OrgTransactionFactory,
		a.OrganizationId,
		func(tx repositories.Transaction) (any, error) {
			result, err := a.IngestedDataReadRepository.QueryAggregatedValue(tx, tableName, fieldName, aggregator, filters)
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
