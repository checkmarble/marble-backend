package scheduled_execution

import (
	"fmt"
	"slices"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/models/ast"
	"github.com/checkmarble/marble-backend/pure_utils/duration"

	"github.com/jackc/pgx/v5"
)

func selectFiltersFromTriggerAstRootAnd(node ast.Node, table models.TableIdentifier) []models.Filter {
	if node.Function != ast.FUNC_AND {
		return nil
	}

	filters := make([]models.Filter, 0, 10)
	for _, node := range node.Children {
		filter, valid := filterFromComparisonNode(node, table, 0)
		onlyConstantValues := filterHasOnlyConstantValues(filter)
		if valid && !onlyConstantValues {
			filters = append(filters, filter)
		}
	}
	return filters
}

// We ignore filters that only use constant values. Indeed, they are not most useful for filtering the data, and
// they generate errors once translated to SQL (we pass the values as 'any' to pgx, it needs typehints to compare params
// if it has no hint from columns or SQL expressions with which we compare them).
func filterHasOnlyConstantValues(filter models.Filter) bool {
	leftConstant := filter.LeftValue != nil || (filter.LeftNestedFilter != nil &&
		filterHasOnlyConstantValues(*filter.LeftNestedFilter))
	rightConstant := filter.RightValue != nil || (filter.RightNestedFilter != nil &&
		filterHasOnlyConstantValues(*filter.RightNestedFilter))

	return leftConstant && rightConstant
}

func filterFromComparisonNode(node ast.Node, table models.TableIdentifier, depth int) (models.Filter, bool) {
	if depth > 3 {
		// Should not happen on a scenario edited in the app, but to be sure: avoid too deep recursion
		return models.Filter{}, false
	}

	leftVal := comparisonValueFromNode(node.Children[0], table, depth)
	if !leftVal.valid {
		return models.Filter{}, false
	}

	var rightVal parsedFilterValue
	if len(node.Children) == 2 {
		rightVal = comparisonValueFromNode(node.Children[1], table, depth)
		if !rightVal.valid {
			return models.Filter{}, false
		}
	}

	return models.Filter{
		LeftSql:           leftVal.rawSql,
		LeftValue:         leftVal.value,
		LeftNestedFilter:  leftVal.nestedValue,
		Operator:          node.Function,
		RightSql:          rightVal.rawSql,
		RightValue:        rightVal.value,
		RightNestedFilter: rightVal.nestedValue,
	}, true
}

type parsedFilterValue struct {
	rawSql      string
	value       any
	nestedValue *models.Filter
	valid       bool
}

func comparisonValueFromNode(node ast.Node, table models.TableIdentifier, depth int) parsedFilterValue {
	// case 1: payload field
	if node.Function == ast.FUNC_PAYLOAD {
		field, ok := payloadFieldNodeToSql(node, table)
		return parsedFilterValue{rawSql: field, valid: ok}
	}

	// case 2: date add operator
	if node.Function == ast.FUNC_TIME_ADD {
		timeNode := node.NamedChildren["timestampField"]
		var time string
		if timeNode.Function == ast.FUNC_TIME_NOW {
			time = "now()"
		} else if timeNode.Function == ast.FUNC_PAYLOAD {
			field, ok := payloadFieldNodeToSql(timeNode, table)
			if !ok {
				return parsedFilterValue{}
			}
			time = field
		} else {
			return parsedFilterValue{}
		}

		sign, ok := node.NamedChildren["sign"].Constant.(string)
		if !ok || (sign != "+" && sign != "-") {
			return parsedFilterValue{}
		}
		durationStr, ok := node.NamedChildren["duration"].Constant.(string)
		if !ok {
			return parsedFilterValue{}
		}
		if _, err := duration.Parse(durationStr); err != nil {
			// check against possible SQL injection
			return parsedFilterValue{}
		}

		return parsedFilterValue{
			rawSql: fmt.Sprintf("%s %s interval '%s'", time, sign, durationStr),
			valid:  true,
		}
	}

	// case 3: now operator
	if node.Function == ast.FUNC_TIME_NOW {
		return parsedFilterValue{rawSql: "now()", valid: true}
	}

	// case 4: constant value: string number, boolean or list of strings
	if node.Constant != nil {
		switch node.Constant.(type) {
		case string, float64, bool:
			return parsedFilterValue{value: node.Constant, valid: true}
		case []any:
			// if the json ast contains an array of strings, is is decoded as []any by go
			strings := []string{}
			for _, v := range node.Constant.([]any) {
				if s, ok := v.(string); ok {
					strings = append(strings, s)
				} else {
					return parsedFilterValue{}
				}
			}
			return parsedFilterValue{value: strings, valid: true}
		default:
			return parsedFilterValue{}
		}
	}

	// case 5: nested operator (mathematical operation or comparison)
	if canBeNestedNode(node) {
		nestedFilter, valid := filterFromComparisonNode(node, table, depth+1)
		return parsedFilterValue{nestedValue: &nestedFilter, valid: valid}
	}

	// return no filter in all other cases, in particular: DB field access, aggregates, custom list access, string similarity
	return parsedFilterValue{}
}

func payloadFieldNodeToSql(node ast.Node, table models.TableIdentifier) (string, bool) {
	field, ok := node.Children[0].Constant.(string)
	return pgx.Identifier.Sanitize([]string{table.Schema, table.Table, field}), ok
}

func canBeNestedNode(node ast.Node) bool {
	return slices.Contains(
		[]ast.Function{
			ast.FUNC_GREATER,
			ast.FUNC_GREATER_OR_EQUAL,
			ast.FUNC_LESS,
			ast.FUNC_LESS_OR_EQUAL,
			ast.FUNC_EQUAL,
			ast.FUNC_NOT_EQUAL,
			ast.FUNC_IS_IN_LIST,
			ast.FUNC_IS_NOT_IN_LIST,
			ast.FUNC_STRING_CONTAINS,
			ast.FUNC_STRING_NOT_CONTAIN,
			ast.FUNC_SUBTRACT,
			ast.FUNC_ADD,
			ast.FUNC_MULTIPLY,
			ast.FUNC_DIVIDE,
			ast.FUNC_IS_EMPTY,
			ast.FUNC_IS_NOT_EMPTY,
		},
		node.Function)
}
