package scheduled_execution

import (
	"slices"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/models/ast"
)

func selectFiltersFromTriggerAstRootAnd(node ast.Node) ([]models.Filter, error) {
	if node.Function != ast.FUNC_AND {
		return nil, nil
	}

	filters := make([]models.Filter, 0, 10)
	for _, node := range node.Children {
		filter, valid := astConditionToFilter(node)
		if valid {
			filters = append(filters, filter)
		}
	}
	return filters, nil
}

func astConditionToFilter(node ast.Node) (models.Filter, bool) {
	switch node.Function {
	case ast.FUNC_GREATER,
		ast.FUNC_GREATER_OR_EQUAL,
		ast.FUNC_LESS,
		ast.FUNC_LESS_OR_EQUAL,
		ast.FUNC_EQUAL,
		ast.FUNC_NOT_EQUAL:
		return models.Filter{}, false
	case ast.FUNC_IS_IN_LIST,
		ast.FUNC_IS_NOT_IN_LIST,
		ast.FUNC_CONTAINS_ANY,
		ast.FUNC_CONTAINS_NONE:
		return models.Filter{}, false
	case ast.FUNC_STRING_CONTAINS,
		ast.FUNC_STRING_NOT_CONTAIN:
		return models.Filter{}, false
	}
	return models.Filter{}, false
}

func filterFromComparisonNode(node ast.Node) (models.Filter, bool) {
	if !slices.Contains([]ast.Function{
		ast.FUNC_GREATER,
		ast.FUNC_GREATER_OR_EQUAL,
		ast.FUNC_LESS,
		ast.FUNC_LESS_OR_EQUAL,
		ast.FUNC_EQUAL,
		ast.FUNC_NOT_EQUAL,
	}, node.Function) {
		return models.Filter{}, false
	}

	// left options:
	// - constant (string, number, boolean allowed. String can be timestamp)
	// - payload (field name)
	// - time_add operator with something else inside
	// - "now" operator

	// right options:
	// same things actually, the comparison is "symmetric"

	// one at least must involve a "payload" field, otherwise nothing to filter on the table

	fieldName, err := node.ReadConstantNamedChildString("fieldName")
	if err != nil {
		return models.Filter{}, false
	}

	value, err := node.ReadConstantNamedChildString("value")
	if err != nil {
		return models.Filter{}, false
	}

	return models.Filter{
		LeftFieldOrValue:  nil,
		Operator:          node.Function,
		RightFieldOrValue: nil,
	}, true
}

func comparisonValueFromNode(node ast.Node) (value any, involvesPayload bool, valid bool) {
	// case 1: payload field

	// case 2: date add operator

	// case 3: now operator

	// case 4: custom list accessor

	// case 5: constant value
	//   - string
	//   - number
	//   - boolean
	//   - timestamp, but it's a string (we can pass the string to the sql query)
	//   - list of strings => should not happen here because it's a comparison operator

	return 0, false, false
}

// func aggregationNodeToQueryFamily(node ast.Node) (models.AggregateQueryFamily, error) {
// 	if node.Function != ast.FUNC_AGGREGATOR {
// 		return models.AggregateQueryFamily{}, errors.Wrap(models.ErrInvalidAST, "Node is not an aggregator")
// 	}

// 	queryTableName, err := node.ReadConstantNamedChildString("tableName")
// 	if err != nil {
// 		return models.AggregateQueryFamily{}, errors.Wrap(models.ErrInvalidAST,
// 			"Error reading tableName in aggregation node: "+err.Error())
// 	}

// 	aggregatedFieldName, err := node.ReadConstantNamedChildString("fieldName")
// 	if err != nil {
// 		return models.AggregateQueryFamily{}, errors.Wrap(models.ErrInvalidAST,
// 			"Error reading fieldName in aggregation node: "+err.Error())
// 	}

// 	family := models.NewAggregateQueryFamily(queryTableName)

// 	filters, ok := node.NamedChildren["filters"]
// 	if !ok {
// 		return family, nil
// 	}
// 	for _, filter := range filters.Children {
// 		if tableNameStr, err := filter.ReadConstantNamedChildString("tableName"); err != nil {
// 			return models.AggregateQueryFamily{}, errors.Wrap(models.ErrInvalidAST,
// 				"Error reading tableName in filter node: "+err.Error())
// 		} else if tableNameStr == "" || tableNameStr != queryTableName {
// 			return models.AggregateQueryFamily{}, errors.Wrap(models.ErrInvalidAST,
// 				"Filter tableName empty or is different from parent aggregator node's tableName")
// 		}

// 		fieldName, err := filter.ReadConstantNamedChildString("fieldName")
// 		if err != nil {
// 			return models.AggregateQueryFamily{}, errors.Wrap(models.ErrInvalidAST,
// 				"Error reading fieldName in filter node: "+err.Error())
// 		} else if fieldName == "" {
// 			return models.AggregateQueryFamily{}, errors.New("Filter fieldName is empty")
// 		}

// 		operatorStr, err := filter.ReadConstantNamedChildString("operator")
// 		if err != nil {
// 			return models.AggregateQueryFamily{}, errors.Wrap(models.ErrInvalidAST,
// 				"Error reading operator in filter node:"+err.Error())
// 		}

// 		switch ast.FilterOperator(operatorStr) {
// 		case ast.FILTER_EQUAL:
// 			family.EqConditions.Insert(fieldName)
// 		case ast.FILTER_GREATER, ast.FILTER_GREATER_OR_EQUAL, ast.FILTER_LESSER, ast.FILTER_LESSER_OR_EQUAL:
// 			if !family.EqConditions.Contains(fieldName) {
// 				family.IneqConditions.Insert(fieldName)
// 			}
// 		case ast.FILTER_IS_IN_LIST, ast.FILTER_IS_NOT_IN_LIST, ast.FILTER_NOT_EQUAL:
// 			if !family.EqConditions.Contains(fieldName) &&
// 				!family.IneqConditions.Contains(fieldName) {
// 				family.SelectOrOtherConditions.Insert(fieldName)
// 			}
// 		default:
// 			return models.AggregateQueryFamily{}, errors.Wrap(models.ErrInvalidAST,
// 				fmt.Sprintf("Filter operator %s is not valid", operatorStr))
// 		}
// 	}

// 	// Columns that are used in the index but not in = or <,>,>=,<= filters are added as columns to be "included" in the index
// 	if !family.EqConditions.Contains(aggregatedFieldName) &&
// 		!family.IneqConditions.Contains(aggregatedFieldName) {
// 		family.SelectOrOtherConditions.Insert(aggregatedFieldName)
// 	}

// 	return family, nil
// }
