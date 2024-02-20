package indexes

import (
	"fmt"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/models/ast"
	"github.com/hashicorp/go-set/v2"
	"github.com/pkg/errors"
)

func indexesToCreateFromScenarioIterations(
	scenarioIterations []models.ScenarioIteration,
	existingIndexes []models.ConcreteIndex,
) ([]models.ConcreteIndex, error) {
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
func extractQueryFamiliesFromAstSlice(nodes []ast.Node) (*set.HashSet[models.AggregateQueryFamily, string], error) {
	families := set.NewHashSet[models.AggregateQueryFamily, string](0)

	for _, node := range nodes {
		nodeFamilies, err := extractQueryFamiliesFromAst(node)
		if err != nil {
			return nil, errors.Wrap(err, "Error getting query families from node")
		}
		families = families.Union(nodeFamilies).(*set.HashSet[models.AggregateQueryFamily, string])
	}

	return families, nil
}

func extractQueryFamiliesFromAst(node ast.Node) (set.Collection[models.AggregateQueryFamily], error) {
	families := set.NewHashSet[models.AggregateQueryFamily, string](0)

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
		families = families.Union(childFamilies).(*set.HashSet[models.AggregateQueryFamily, string])
	}
	for _, child := range node.NamedChildren {
		childFamilies, err := extractQueryFamiliesFromAst(child)
		if err != nil {
			return nil, errors.Wrap(err, "Error getting query families from named child")
		}
		families = families.Union(childFamilies).(*set.HashSet[models.AggregateQueryFamily, string])
	}

	return families, nil
}

func aggregationNodeToQueryFamily(node ast.Node) (models.AggregateQueryFamily, error) {
	if node.Function != ast.FUNC_AGGREGATOR {
		return models.AggregateQueryFamily{}, errors.New("Node is not an aggregator")
	}

	queryTableName, err := node.ReadConstantNamedChildString("tableName")
	if err != nil {
		return models.AggregateQueryFamily{}, errors.Wrap(err,
			"Error reading tableName in aggregation node")
	}

	aggregatedFieldNameStr, err := node.ReadConstantNamedChildString("fieldName")
	if err != nil {
		return models.AggregateQueryFamily{}, errors.Wrap(err,
			"Error reading fieldName in aggregation node")
	}
	aggregatedFieldName := models.FieldName(aggregatedFieldNameStr)

	family := models.AggregateQueryFamily{
		TableName:               models.TableName(queryTableName),
		EqConditions:            set.New[models.FieldName](0),
		IneqConditions:          set.New[models.FieldName](0),
		SelectOrOtherConditions: set.New[models.FieldName](0),
	}

	filters, ok := node.NamedChildren["filters"]
	if !ok {
		return family, nil
	}
	for _, filter := range filters.Children {
		if tableNameStr, err := filter.ReadConstantNamedChildString("tableName"); err != nil {
			return models.AggregateQueryFamily{}, errors.Wrap(err,
				"Error reading tableName in filter node")
		} else if tableNameStr == "" || tableNameStr != queryTableName {
			return models.AggregateQueryFamily{}, errors.New(
				"Filter tableName empty or is different from parent aggregator node's tableName")
		}

		fieldNameStr, err := filter.ReadConstantNamedChildString("fieldName")
		if err != nil {
			return models.AggregateQueryFamily{}, errors.Wrap(err,
				"Error reading fieldName in filter node")
		} else if fieldNameStr == "" {
			return models.AggregateQueryFamily{}, errors.New("Filter fieldName is empty")
		}
		fieldName := models.FieldName(fieldNameStr)

		operatorStr, err := filter.ReadConstantNamedChildString("operator")
		if err != nil {
			return models.AggregateQueryFamily{}, errors.Wrap(err,
				"Error reading operator in filter node")
		}

		switch ast.FilterOperator(operatorStr) {
		case ast.FILTER_EQUAL:
			family.EqConditions.Insert(fieldName)
		case ast.FILTER_GREATER, ast.FILTER_GREATER_OR_EQUAL, ast.FILTER_LESSER, ast.FILTER_LESSER_OR_EQUAL:
			if !family.EqConditions.Contains(fieldName) {
				family.IneqConditions.Insert(fieldName)
			}
		case ast.FILTER_IS_IN_LIST, ast.FILTER_IS_NOT_IN_LIST, ast.FILTER_NOT_EQUAL:
			if !family.EqConditions.Contains(fieldName) &&
				!family.IneqConditions.Contains(fieldName) {
				family.SelectOrOtherConditions.Insert(fieldName)
			}
		default:
			return models.AggregateQueryFamily{}, errors.New(
				fmt.Sprintf("Filter operator %s is not valid", operatorStr))
		}
	}

	// Columns that are used in the index but not in = or <,>,>=,<= filters are added as columns to be "included" in the index
	if !family.EqConditions.Contains(aggregatedFieldName) &&
		!family.IneqConditions.Contains(aggregatedFieldName) {
		family.SelectOrOtherConditions.Insert(aggregatedFieldName)
	}

	return family, nil
}

func indexesToCreateFromQueryFamilies(
	queryFamilies set.Collection[models.AggregateQueryFamily],
	existingIndexes []models.ConcreteIndex,
) []models.ConcreteIndex {
	familiesToCreate := set.NewHashSet[models.IndexFamily, string](0)
	for _, q := range queryFamilies.Slice() {
		familiesToCreate = familiesToCreate.Union(
			selectIdxFamiliesToCreate(q.ToIndexFamilies(), existingIndexes),
		).(*set.HashSet[models.IndexFamily, string])
	}
	reducedFamiliesToCreate := extractMinimalSetOfIdxFamilies(familiesToCreate)
	return selectConcreteIndexesToCreate(reducedFamiliesToCreate, existingIndexes)
}
