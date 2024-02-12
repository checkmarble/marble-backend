package models

import (
	"fmt"
	"slices"

	"github.com/cockroachdb/errors"
	"github.com/hashicorp/go-set/v2"

	"github.com/checkmarble/marble-backend/models/ast"
)

type AggregateQueryFamily struct {
	TableName               TableName
	EqConditions            *set.Set[FieldName]
	IneqConditions          *set.Set[FieldName]
	SelectOrOtherConditions *set.Set[FieldName]
}

func (family AggregateQueryFamily) Equal(other AggregateQueryFamily) bool {
	return family.TableName == other.TableName &&
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
	return fmt.Sprintf("%s - %s - %s - %s", family.TableName, eq, ineq, other)
}

func IndexesToCreateFromScenarioIterations(
	scenarioIterations []ScenarioIteration,
	existingIndexes []ConcreteIndex,
) ([]ConcreteIndex, error) {
	var asts []ast.Node
	for _, i := range scenarioIterations {
		asts = append(asts, *i.TriggerConditionAstExpression)
		for _, r := range i.Rules {
			asts = append(asts, *r.FormulaAstExpression)
		}
	}

	queryFamilies, err := extractQueryFamiliesFromAstSlice(asts)
	if err != nil {
		return nil, errors.Wrap(err, "Error extracting query families from scenario iterations")
	}

	return indexesToCreateFromQueryFamilies(queryFamilies, existingIndexes), nil
}

// simple utility function using extractQueryFamiliesFromAst above
func extractQueryFamiliesFromAstSlice(nodes []ast.Node) (*set.HashSet[AggregateQueryFamily, string], error) {
	families := set.NewHashSet[AggregateQueryFamily, string](0)

	for _, node := range nodes {
		nodeFamilies, err := extractQueryFamiliesFromAst(node)
		if err != nil {
			return nil, errors.Wrap(err, "Error getting query families from node")
		}
		families = families.Union(nodeFamilies).(*set.HashSet[AggregateQueryFamily, string])
	}

	return families, nil
}

func extractQueryFamiliesFromAst(node ast.Node) (set.Collection[AggregateQueryFamily], error) {
	families := set.NewHashSet[AggregateQueryFamily, string](0)

	if node.Function == ast.FUNC_AGGREGATOR {
		family, err := aggregationNodeToQueryFamily(node)
		if err != nil {
			return nil, errors.Wrap(err, "Error converting aggregation node to query family")
		}
		families.Insert(family)
	}

	// union with query families from all children
	for _, child := range node.Children {
		childFamilies, err := extractQueryFamiliesFromAst(child)
		if err != nil {
			return nil, errors.Wrap(err, "Error getting query families from child")
		}
		families = families.Union(childFamilies).(*set.HashSet[AggregateQueryFamily, string])
	}
	for _, child := range node.NamedChildren {
		childFamilies, err := extractQueryFamiliesFromAst(child)
		if err != nil {
			return nil, errors.Wrap(err, "Error getting query families from named child")
		}
		families = families.Union(childFamilies).(*set.HashSet[AggregateQueryFamily, string])
	}

	return families, nil
}

func aggregationNodeToQueryFamily(node ast.Node) (AggregateQueryFamily, error) {
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
		TableName:               TableName(queryTableName),
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

func indexesToCreateFromQueryFamilies(
	queryFamilies set.Collection[AggregateQueryFamily],
	existingIndexes []ConcreteIndex,
) []ConcreteIndex {
	familiesToCreate := set.NewHashSet[IndexFamily, string](0)
	for _, q := range queryFamilies.Slice() {
		familiesToCreate = familiesToCreate.Union(
			selectIdxFamiliesToCreate(q.toIndexFamilies(), existingIndexes),
		).(*set.HashSet[IndexFamily, string])
	}
	reducedFamiliesToCreate := extractMinimalSetOfIdxFamilies(familiesToCreate)
	return selectConcreteIndexesToCreate(reducedFamiliesToCreate, existingIndexes)
}

func (qFamily AggregateQueryFamily) toIndexFamilies() *set.HashSet[IndexFamily, string] {
	return aggregateQueryToIndexFamily(qFamily)
}

func aggregateQueryToIndexFamily(qFamily AggregateQueryFamily) *set.HashSet[IndexFamily, string] {
	// we output a collection of index families, with the different combinations of "inequality filtering"
	//  at the end of the index.
	// E.g. if we have a query with conditions a = 1, b = 2, c > 3, d > 4, e > 5, we output:
	// { Flex: {a,b}, Last: c, Included: {d,e} }  +  { Flex: {a,b}, Last: d, Included: {c,e} }   +  { Flex: {a,b}, Last: e, Included: {c,d} }
	output := set.NewHashSet[IndexFamily, string](0)

	// first iterate on equality conditions and colunms to include anyway
	base := NewIndexFamily()
	base.TableName = qFamily.TableName
	if qFamily.EqConditions != nil {
		qFamily.EqConditions.ForEach(func(f FieldName) bool {
			base.Flex.Insert(f)
			return true
		})
	}
	if qFamily.SelectOrOtherConditions != nil {
		qFamily.SelectOrOtherConditions.ForEach(func(f FieldName) bool {
			base.Included.Insert(f)
			return true
		})
	}
	if qFamily.IneqConditions == nil || qFamily.IneqConditions.Size() == 0 {
		output.Insert(base)
		return output
	}

	// If inequality conditions are involved, we need to create a family for each column involved
	// in the inequality conditions (and complete the "other" columns)
	qFamily.IneqConditions.ForEach(func(f FieldName) bool {
		// we create a copy of the base family
		family := base.copy()
		// we add the current column as the "last" column
		family.Last = f
		// we add all the other columns as "other" columns
		qFamily.IneqConditions.ForEach(func(o FieldName) bool {
			if o != f {
				family.Included.Insert(o)
			}
			return true
		})
		output.Insert(family)
		return true
	})

	return output
}
