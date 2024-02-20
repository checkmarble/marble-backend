package indexes

import (
	"context"
	"fmt"
	"testing"

	"github.com/hashicorp/go-set/v2"
	"github.com/stretchr/testify/assert"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/models/ast"
)

func TestAggregationNodeToQueryFamily(t *testing.T) {
	t.Run("empty filters", func(t *testing.T) {
		asserts := assert.New(t)
		node := ast.Node{
			Function: ast.FUNC_AGGREGATOR,
			NamedChildren: map[string]ast.Node{
				"tableName": ast.NewNodeConstant("table"),
				"fieldName": ast.NewNodeConstant("field 0"),
				"filters": {
					Children: []ast.Node{},
				},
			},
		}
		aggregateFamily, err := aggregationNodeToQueryFamily(node)
		asserts.NoError(err)
		asserts.Equal(models.TableName("table"), aggregateFamily.TableName,
			"table name should be input table name")
	})

	t.Run("missing filters child", func(t *testing.T) {
		asserts := assert.New(t)
		node := ast.Node{
			Function: ast.FUNC_AGGREGATOR,
			NamedChildren: map[string]ast.Node{
				"tableName": ast.NewNodeConstant("table"),
				"fieldName": ast.NewNodeConstant("field 0"),
			},
		}
		aggregateFamily, err := aggregationNodeToQueryFamily(node)
		asserts.NoError(err)
		asserts.Equal(0, aggregateFamily.EqConditions.Size(), "EqConditions should be empty")
		asserts.Equal(0, aggregateFamily.IneqConditions.Size(), "IneqConditions should be empty")
		asserts.Equal(0, aggregateFamily.SelectOrOtherConditions.Size(),
			"No index is relevant if there are no filters")
	})

	t.Run("with one = filter", func(t *testing.T) {
		asserts := assert.New(t)
		node := ast.Node{
			Function: ast.FUNC_AGGREGATOR,
			NamedChildren: map[string]ast.Node{
				"tableName": ast.NewNodeConstant("table"),
				"fieldName": ast.NewNodeConstant("field 0"),
				"filters": {
					Children: []ast.Node{
						{
							Function: ast.FUNC_FILTER,
							NamedChildren: map[string]ast.Node{
								"tableName": ast.NewNodeConstant("table"),
								"fieldName": ast.NewNodeConstant("field"),
								"operator":  ast.NewNodeConstant("="),
							},
						},
					},
				},
			},
		}
		aggregateFamily, err := aggregationNodeToQueryFamily(node)
		asserts.NoError(err)
		asserts.Equal(models.TableName("table"), aggregateFamily.TableName)
		asserts.Equal(1, aggregateFamily.EqConditions.Size())
		asserts.True(aggregateFamily.EqConditions.Contains(models.FieldName("field")))
		asserts.Equal(0, aggregateFamily.IneqConditions.Size())
		asserts.Equal(1, aggregateFamily.SelectOrOtherConditions.Size(),
			"SelectOrOtherConditions should contain field 0")
	})

	t.Run("with one = and one > filter on same field", func(t *testing.T) {
		asserts := assert.New(t)
		node := ast.Node{
			Function: ast.FUNC_AGGREGATOR,
			NamedChildren: map[string]ast.Node{
				"tableName": ast.NewNodeConstant("table"),
				"fieldName": ast.NewNodeConstant("field 0"),
				"filters": {
					Children: []ast.Node{
						{
							Function: ast.FUNC_FILTER,
							NamedChildren: map[string]ast.Node{
								"tableName": ast.NewNodeConstant("table"),
								"fieldName": ast.NewNodeConstant("field"),
								"operator":  ast.NewNodeConstant("="),
							},
						},
						{
							Function: ast.FUNC_FILTER,
							NamedChildren: map[string]ast.Node{
								"tableName": ast.NewNodeConstant("table"),
								"fieldName": ast.NewNodeConstant("field"),
								"operator":  ast.NewNodeConstant(">"),
							},
						},
					},
				},
			},
		}
		aggregateFamily, err := aggregationNodeToQueryFamily(node)
		asserts.NoError(err)
		asserts.Equal(models.TableName("table"), aggregateFamily.TableName)
		asserts.Equal(1, aggregateFamily.EqConditions.Size())
		asserts.True(aggregateFamily.EqConditions.Contains(models.FieldName("field")))
		asserts.Equal(0, aggregateFamily.IneqConditions.Size())
		asserts.Equal(1, aggregateFamily.SelectOrOtherConditions.Size(),
			"SelectOrOtherConditions should contain field 0")
	})

	t.Run("with one = and one > filter on different fields", func(t *testing.T) {
		asserts := assert.New(t)
		node := ast.Node{
			Function: ast.FUNC_AGGREGATOR,
			NamedChildren: map[string]ast.Node{
				"tableName": ast.NewNodeConstant("table"),
				"fieldName": ast.NewNodeConstant("field 0"),
				"filters": {
					Children: []ast.Node{
						{
							Function: ast.FUNC_FILTER,
							NamedChildren: map[string]ast.Node{
								"tableName": ast.NewNodeConstant("table"),
								"fieldName": ast.NewNodeConstant("field 1"),
								"operator":  ast.NewNodeConstant("="),
							},
						},
						{
							Function: ast.FUNC_FILTER,
							NamedChildren: map[string]ast.Node{
								"tableName": ast.NewNodeConstant("table"),
								"fieldName": ast.NewNodeConstant("field 2"),
								"operator":  ast.NewNodeConstant(">"),
							},
						},
					},
				},
			},
		}
		aggregateFamily, err := aggregationNodeToQueryFamily(node)
		asserts.NoError(err)
		asserts.Equal(models.TableName("table"), aggregateFamily.TableName)
		asserts.Equal(1, aggregateFamily.EqConditions.Size())
		asserts.True(aggregateFamily.EqConditions.Contains(models.FieldName("field 1")))
		asserts.Equal(1, aggregateFamily.IneqConditions.Size())
		asserts.True(aggregateFamily.IneqConditions.Contains(models.FieldName("field 2")))
		asserts.Equal(1, aggregateFamily.SelectOrOtherConditions.Size(),
			"SelectOrOtherConditions should contain field 0")
	})

	t.Run("with invalid filter", func(t *testing.T) {
		asserts := assert.New(t)
		node := ast.Node{
			Function: ast.FUNC_AGGREGATOR,
			NamedChildren: map[string]ast.Node{
				"tableName": ast.NewNodeConstant("table"),
				"fieldName": ast.NewNodeConstant("field 0"),
				"filters": {
					Children: []ast.Node{
						{
							Function: ast.FUNC_FILTER,
							NamedChildren: map[string]ast.Node{
								"tableName": ast.NewNodeConstant("table"),
								"fieldName": ast.NewNodeConstant("field"),
								"operator":  ast.NewNodeConstant("THIS IS NOT A FILTER"),
							},
						},
					},
				},
			},
		}
		_, err := aggregationNodeToQueryFamily(node)
		asserts.Error(err)
	})

	t.Run("Most general case", func(t *testing.T) {
		asserts := assert.New(t)
		node := ast.Node{
			Function: ast.FUNC_AGGREGATOR,
			NamedChildren: map[string]ast.Node{
				"tableName": ast.NewNodeConstant("table"),
				"fieldName": ast.NewNodeConstant("field 0"),
				"filters": {
					Children: []ast.Node{
						{
							Function: ast.FUNC_FILTER,
							NamedChildren: map[string]ast.Node{
								"tableName": ast.NewNodeConstant("table"),
								"fieldName": ast.NewNodeConstant("field 1"),
								"operator":  ast.NewNodeConstant("="),
							},
						},
						{
							Function: ast.FUNC_FILTER,
							NamedChildren: map[string]ast.Node{
								"tableName": ast.NewNodeConstant("table"),
								"fieldName": ast.NewNodeConstant("field 2"),
								"operator":  ast.NewNodeConstant("="),
							},
						},
						{
							Function: ast.FUNC_FILTER,
							NamedChildren: map[string]ast.Node{
								"tableName": ast.NewNodeConstant("table"),
								"fieldName": ast.NewNodeConstant("field 3"),
								"operator":  ast.NewNodeConstant("<"),
							},
						},
						{
							Function: ast.FUNC_FILTER,
							NamedChildren: map[string]ast.Node{
								"tableName": ast.NewNodeConstant("table"),
								"fieldName": ast.NewNodeConstant("field 1"),
								"operator":  ast.NewNodeConstant("IsNotInList"),
							},
						},
					},
				},
			},
		}
		aggregateFamily, err := aggregationNodeToQueryFamily(node)
		asserts.NoError(err)
		asserts.Equal(models.TableName("table"), aggregateFamily.TableName)
		asserts.Equal(2, aggregateFamily.EqConditions.Size(),
			"EqConditions should contain field 1 and field 2")
		asserts.True(aggregateFamily.EqConditions.Contains(models.FieldName("field 1")), "EqConditions should contain field 1")
		asserts.True(aggregateFamily.EqConditions.Contains(models.FieldName("field 2")), "EqConditions should contain field 2")
		asserts.Equal(1, aggregateFamily.IneqConditions.Size(),
			"IneqConditions should contain field 3")
		asserts.True(aggregateFamily.IneqConditions.Contains(models.FieldName("field 3")),
			"IneqConditions should contain field 3")
		asserts.Equal(1, aggregateFamily.SelectOrOtherConditions.Size(),
			"SelectOrOtherConditions should contain 1 field")
		asserts.True(aggregateFamily.SelectOrOtherConditions.Contains(
			models.FieldName("field 0")), "SelectOrOtherConditions should contain field 0")
	})
}

func TestAstNodeToQueryFamilies(t *testing.T) {
	t.Run("empty node", func(t *testing.T) {
		asserts := assert.New(t)
		output, err := extractQueryFamiliesFromAst(context.Background(), ast.Node{})
		asserts.NoError(err)
		asserts.Equal(0, output.Size())
	})

	t.Run("just one nested aggregation with duplicate filter", func(t *testing.T) {
		asserts := assert.New(t)
		ast := ast.Node{
			Function: ast.FUNC_AND,
			Children: []ast.Node{
				{
					Function: ast.FUNC_AGGREGATOR,
					NamedChildren: map[string]ast.Node{
						"tableName": ast.NewNodeConstant("table"),
						"fieldName": ast.NewNodeConstant("field 0"),
						"filters": {
							Children: []ast.Node{{
								Function: ast.FUNC_FILTER,
								NamedChildren: map[string]ast.Node{
									"tableName": ast.NewNodeConstant("table"),
									"fieldName": ast.NewNodeConstant("field"),
									"operator":  ast.NewNodeConstant("="),
								},
							}},
						},
					},
				},
				{
					Function: ast.FUNC_AGGREGATOR,
					NamedChildren: map[string]ast.Node{
						"tableName": ast.NewNodeConstant("table"),
						"fieldName": ast.NewNodeConstant("field 0"),
						"filters": {
							Children: []ast.Node{{
								Function: ast.FUNC_FILTER,
								NamedChildren: map[string]ast.Node{
									"tableName": ast.NewNodeConstant("table"),
									"fieldName": ast.NewNodeConstant("field"),
									"operator":  ast.NewNodeConstant("="),
								},
							}},
						},
					},
				},
			},
		}
		output, err := extractQueryFamiliesFromAst(context.Background(), ast)
		asserts.NoError(err)
		asserts.Equal(1, output.Size(), "There should be only 1 query family in the output set")
		expected := set.NewHashSet[models.AggregateQueryFamily](0)
		expected.Insert(models.AggregateQueryFamily{
			TableName:               models.TableName("table"),
			EqConditions:            set.From[models.FieldName]([]models.FieldName{"field"}),
			IneqConditions:          set.New[models.FieldName](0),
			SelectOrOtherConditions: set.From[models.FieldName]([]models.FieldName{"field 0"}),
		})
		asserts.True(output.EqualSet(expected), "The output set should contain the one query family (that was present twice)")
		fmt.Println(expected)
		fmt.Println(output)
	})

	t.Run("nominal case with nesting, several layers, duplicates", func(t *testing.T) {
		asserts := assert.New(t)
		ast := ast.Node{
			Children: []ast.Node{
				{
					Function: ast.FUNC_AGGREGATOR,
					NamedChildren: map[string]ast.Node{
						"tableName": ast.NewNodeConstant("table"),
						"fieldName": ast.NewNodeConstant("field 0"),
						"filters": {
							Children: []ast.Node{
								{
									Function: ast.FUNC_FILTER,
									NamedChildren: map[string]ast.Node{
										"tableName": ast.NewNodeConstant("table"),
										"fieldName": ast.NewNodeConstant("field"),
										"operator":  ast.NewNodeConstant("="),
									},
								},
							},
						},
						"value": {
							Function: ast.FUNC_AGGREGATOR,
							NamedChildren: map[string]ast.Node{
								"tableName": ast.NewNodeConstant("table"),
								"fieldName": ast.NewNodeConstant("field 2"),
								"filters": {
									Children: []ast.Node{
										{
											Function: ast.FUNC_FILTER,
											NamedChildren: map[string]ast.Node{
												"tableName": ast.NewNodeConstant("table"),
												"fieldName": ast.NewNodeConstant("field 0"),
												"operator":  ast.NewNodeConstant("="),
											},
										},
										{
											Function: ast.FUNC_FILTER,
											NamedChildren: map[string]ast.Node{
												"tableName": ast.NewNodeConstant("table"),
												"fieldName": ast.NewNodeConstant("field 3"),
												"operator":  ast.NewNodeConstant("IsInList"),
											},
										},
									},
								},
							},
						},
					},
				},
				{
					Function: ast.FUNC_AGGREGATOR,
					NamedChildren: map[string]ast.Node{
						"tableName": ast.NewNodeConstant("table"),
						"fieldName": ast.NewNodeConstant("field 0"),
						"filters": {
							Children: []ast.Node{
								{
									Function: ast.FUNC_FILTER,
									NamedChildren: map[string]ast.Node{
										"tableName": ast.NewNodeConstant("table"),
										"fieldName": ast.NewNodeConstant("field"),
										"operator":  ast.NewNodeConstant(">"),
									},
								},
							},
						},
					},
				},
			},
		}
		output, err := extractQueryFamiliesFromAst(context.Background(), ast)
		asserts.NoError(err)
		asserts.Equal(3, output.Size(), "There should be 2 query families in the output set")
		expected := set.NewHashSet[models.AggregateQueryFamily](0)
		expected.Insert(models.AggregateQueryFamily{
			TableName:               models.TableName("table"),
			EqConditions:            set.From[models.FieldName]([]models.FieldName{"field"}),
			IneqConditions:          set.New[models.FieldName](0),
			SelectOrOtherConditions: set.From[models.FieldName]([]models.FieldName{"field 0"}),
		})
		expected.Insert(models.AggregateQueryFamily{
			TableName:               models.TableName("table"),
			EqConditions:            set.New[models.FieldName](0),
			IneqConditions:          set.From[models.FieldName]([]models.FieldName{"field"}),
			SelectOrOtherConditions: set.From[models.FieldName]([]models.FieldName{"field 0"}),
		})
		expected.Insert(models.AggregateQueryFamily{
			TableName:      models.TableName("table"),
			EqConditions:   set.From[models.FieldName]([]models.FieldName{"field 0"}),
			IneqConditions: set.New[models.FieldName](0),
			SelectOrOtherConditions: set.From[models.FieldName]([]models.FieldName{
				"field 2", "field 3",
			}),
		})
		asserts.True(output.EqualSet(expected), "The output set should contain the 2 query families")
		fmt.Println(expected)
		fmt.Println(output)
	})
}
