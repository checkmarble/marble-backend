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
		filter, valid := astConditionToFilter(node, table)
		if valid {
			filters = append(filters, filter)
		}
	}
	return filters
}

func astConditionToFilter(node ast.Node, table models.TableIdentifier) (models.Filter, bool) {
	switch node.Function {
	case ast.FUNC_GREATER,
		ast.FUNC_GREATER_OR_EQUAL,
		ast.FUNC_LESS,
		ast.FUNC_LESS_OR_EQUAL,
		ast.FUNC_EQUAL,
		ast.FUNC_NOT_EQUAL:
		return filterFromComparisonNode(node, table)
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

func filterFromComparisonNode(node ast.Node, table models.TableIdentifier) (models.Filter, bool) {
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

	leftVal := comparisonValueFromNode(node.Children[0], table)
	if !leftVal.valid {
		return models.Filter{}, false
	}

	rightVal := comparisonValueFromNode(node.Children[1], table)
	if !rightVal.valid {
		return models.Filter{}, false
	}

	if !leftVal.involvesPayload && !rightVal.involvesPayload {
		return models.Filter{}, false
	}

	return models.Filter{
		LeftSql:    leftVal.rawSql,
		LeftValue:  leftVal.value,
		Operator:   comparisonAstNodeToString(node),
		RightSql:   rightVal.rawSql,
		RightValue: rightVal.value,
	}, true
}

func comparisonAstNodeToString(node ast.Node) string {
	switch node.Function {
	case ast.FUNC_GREATER:
		return ">"
	case ast.FUNC_GREATER_OR_EQUAL:
		return ">="
	case ast.FUNC_LESS:
		return "<"
	case ast.FUNC_LESS_OR_EQUAL:
		return "<="
	case ast.FUNC_EQUAL:
		return "="
	case ast.FUNC_NOT_EQUAL:
		return "!="
	default:
		return ""
	}
}

type parsedFilter struct {
	rawSql          string
	value           any
	involvesPayload bool
	valid           bool
}

func payloadFieldNodeToSql(node ast.Node, table models.TableIdentifier) (string, bool) {
	field, ok := node.Children[0].Constant.(string)
	return pgx.Identifier.Sanitize([]string{table.Schema, table.Table, field}), ok
}

func comparisonValueFromNode(node ast.Node, table models.TableIdentifier) parsedFilter {
	// TODO: handle identifier sanitization against SQL injection

	// case 1: payload field
	if node.Function == ast.FUNC_PAYLOAD {
		field, ok := payloadFieldNodeToSql(node, table)
		return parsedFilter{rawSql: field, involvesPayload: true, valid: ok}
	}

	// case 2: date add operator
	if node.Function == ast.FUNC_TIME_ADD {
		var involvesPayload bool
		timeNode := node.NamedChildren["timestampField"]
		var time string
		if timeNode.Function == ast.FUNC_TIME_NOW {
			time = "now()"
		} else if timeNode.Function == ast.FUNC_PAYLOAD {
			field, ok := payloadFieldNodeToSql(timeNode, table)
			if !ok {
				return parsedFilter{}
			}
			involvesPayload = true
			time = field
		} else {
			return parsedFilter{}
		}

		sign, ok := node.NamedChildren["sign"].Constant.(string)
		if !ok || (sign != "+" && sign != "-") {
			return parsedFilter{}
		}
		durationStr, ok := node.NamedChildren["duration"].Constant.(string)
		if !ok {
			return parsedFilter{}
		}
		if _, err := duration.Parse(durationStr); err != nil {
			// check against possible SQL injection
			return parsedFilter{}
		}

		return parsedFilter{
			rawSql:          fmt.Sprintf("%s %s interval '%s'", time, sign, durationStr),
			involvesPayload: involvesPayload,
			valid:           true,
		}
	}

	// case 3: now operator
	if node.Function == ast.FUNC_TIME_NOW {
		return parsedFilter{rawSql: "now()", valid: true}
	}

	// case 4: constant value
	//   - string
	//   - number
	//   - boolean
	//   - list of strings => should not happen here because it's a comparison operator, ignore it if it does anyway
	if node.Constant != nil {
		// switch on type
		switch node.Constant.(type) {
		case string, float64, bool:
			return parsedFilter{value: node.Constant, valid: true}
		default:
			return parsedFilter{}
		}
	}

	// return no filter in all other cases, in particular aggregates etc.
	return parsedFilter{}
}
