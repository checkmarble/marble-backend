package models

import (
	"fmt"
	"slices"

	"github.com/cockroachdb/errors"
	"github.com/hashicorp/go-set/v2"

	"github.com/checkmarble/marble-backend/models/ast"
)

type AggregateQueryFamily struct {
	Table                   TableName
	EqConditions            *set.Set[FieldName]
	IneqConditions          *set.Set[FieldName]
	SelectOrOtherConditions *set.Set[FieldName]
}

type IndexFamily struct {
	Fixed  []Field
	Flex   *set.Set[FieldName]
	Last   Field
	Others *set.Set[FieldName]
}

func (family AggregateQueryFamily) Equal(other AggregateQueryFamily) bool {
	return family.Table == other.Table &&
		family.EqConditions.Equal(other.EqConditions) &&
		family.IneqConditions.Equal(other.IneqConditions) &&
		family.SelectOrOtherConditions.Equal(other.SelectOrOtherConditions)
}

func (family AggregateQueryFamily) Hash() string {
	// Hash function is used for more easily creating a set of unique query families, taking care of deduplication
	var eq, ineq, other string
	if family.EqConditions == nil {
		eq = ""
	} else {
		s := family.EqConditions.Slice()
		slices.Sort(s)
		eq = fmt.Sprintf("%v", s)
	}
	if family.IneqConditions == nil {
		ineq = ""
	} else {
		s := family.IneqConditions.Slice()
		slices.Sort(s)
		ineq = fmt.Sprintf("%v", s)
	}
	if family.SelectOrOtherConditions == nil {
		other = ""
	} else {
		s := family.SelectOrOtherConditions.Slice()
		slices.Sort(s)
		other = fmt.Sprintf("%v", s)
	}
	return fmt.Sprintf("%s %s %s %s", family.Table, eq, ineq, other)
}

func AggregationNodeToQueryFamily(node ast.Node) (AggregateQueryFamily, error) {
	if node.Function != ast.FUNC_AGGREGATOR {
		return AggregateQueryFamily{}, errors.New("Node is not an aggregator")
	}

	queryTableName, err := node.ReadConstantNamedChildString("tableName")
	if err != nil {
		return AggregateQueryFamily{}, errors.Wrap(err, "Error reading tableName in aggregation node")
	}

	aggregatedFieldNameStr, err := node.ReadConstantNamedChildString("fieldName")
	if err != nil {
		return AggregateQueryFamily{}, errors.Wrap(err, "Error reading fieldName in aggregation node")
	}
	aggregatedFieldName := FieldName(aggregatedFieldNameStr)

	family := AggregateQueryFamily{
		Table:                   TableName(queryTableName),
		EqConditions:            set.New[FieldName](0),
		IneqConditions:          set.New[FieldName](0),
		SelectOrOtherConditions: set.New[FieldName](0),
	}

	filters, ok := node.NamedChildren["filters"]
	if !ok {
		return family, nil
	}
	for _, filter := range filters.Children {
		if tableNameStr, err := filter.ReadConstantNamedChildString("tableName"); err != nil {
			return AggregateQueryFamily{}, errors.Wrap(err, "Error reading tableName in filter node")
		} else if tableNameStr == "" || tableNameStr != queryTableName {
			return AggregateQueryFamily{}, errors.New("Filter tableName empty or is different from parent aggregator node's tableName")
		}

		fieldNameStr, err := filter.ReadConstantNamedChildString("fieldName")
		if err != nil {
			return AggregateQueryFamily{}, errors.Wrap(err, "Error reading fieldName in filter node")
		} else if fieldNameStr == "" {
			return AggregateQueryFamily{}, errors.New("Filter fieldName is empty")
		}
		fieldName := FieldName(fieldNameStr)

		operatorStr, err := filter.ReadConstantNamedChildString("operator")
		if err != nil {
			return AggregateQueryFamily{}, errors.Wrap(err, "Error reading operator in filter node")
		}

		switch ast.FilterOperator(operatorStr) {
		case ast.FILTER_EQUAL:
			family.EqConditions.Insert(fieldName)
		case ast.FILTER_GREATER, ast.FILTER_GREATER_OR_EQUAL, ast.FILTER_LESSER, ast.FILTER_LESSER_OR_EQUAL:
			if !family.EqConditions.Contains(fieldName) {
				family.IneqConditions.Insert(fieldName)
			}
		case ast.FILTER_IS_IN_LIST, ast.FILTER_IS_NOT_IN_LIST, ast.FILTER_NOT_EQUAL:
			if !family.EqConditions.Contains(fieldName) && !family.IneqConditions.Contains(fieldName) {
				family.SelectOrOtherConditions.Insert(fieldName)
			}
		default:
			return AggregateQueryFamily{}, errors.New(fmt.Sprintf("Filter operator %s is not valid", operatorStr))
		}
	}

	// Columns that are used in the index but not in = or <,>,>=,<= filters are added as columns to be "included" in the index
	if !family.EqConditions.Contains(aggregatedFieldName) && !family.IneqConditions.Contains(aggregatedFieldName) {
		family.SelectOrOtherConditions.Insert(aggregatedFieldName)
	}

	return family, nil
}

func ExtractQueryFamiliesFromAst(node ast.Node) (*set.HashSet[AggregateQueryFamily, string], error) {
	families := set.NewHashSet[AggregateQueryFamily, string](0)

	if node.Function == ast.FUNC_AGGREGATOR {
		family, err := AggregationNodeToQueryFamily(node)
		if err != nil {
			return nil, errors.Wrap(err, "Error converting aggregation node to query family")
		}
		families.Insert(family)
	}

	// union with query families from all children
	for _, child := range node.Children {
		childFamilies, err := ExtractQueryFamiliesFromAst(child)
		if err != nil {
			return nil, errors.Wrap(err, "Error getting query families from child")
		}
		families = families.Union(childFamilies).(*set.HashSet[AggregateQueryFamily, string])
	}
	for _, child := range node.NamedChildren {
		childFamilies, err := ExtractQueryFamiliesFromAst(child)
		if err != nil {
			return nil, errors.Wrap(err, "Error getting query families from named child")
		}
		families = families.Union(childFamilies).(*set.HashSet[AggregateQueryFamily, string])
	}

	return families, nil
}

// simple utility function using ExtractQueryFamiliesFromAst above
func ExtractQueryFamiliesFromAstSlice(nodes []ast.Node) (*set.HashSet[AggregateQueryFamily, string], error) {
	families := set.NewHashSet[AggregateQueryFamily, string](0)

	for _, node := range nodes {
		nodeFamilies, err := ExtractQueryFamiliesFromAst(node)
		if err != nil {
			return nil, errors.Wrap(err, "Error getting query families from node")
		}
		families = families.Union(nodeFamilies).(*set.HashSet[AggregateQueryFamily, string])
	}

	return families, nil
}
