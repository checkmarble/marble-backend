package indexes

import (
	"context"
	"fmt"
	"testing"

	"github.com/hashicorp/go-set/v2"
	"github.com/stretchr/testify/assert"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/models/ast"
	"github.com/checkmarble/marble-backend/utils"
)

func makeTestContext() context.Context {
	ctx := context.Background()
	return utils.StoreLoggerInContext(ctx, utils.NewLogger("text"))
}

func TestAggregationNodeToQueryFamily(t *testing.T) {
	dm := models.DataModel{
		Tables: map[string]models.Table{
			"table": {
				Fields: map[string]models.Field{
					"field": {
						Name:         "field",
						PhysicalName: "field",
					},
					"field 0": {
						Name:         "field 0",
						PhysicalName: "field 0",
					},
					"field 1": {
						Name:         "field 1",
						PhysicalName: "field 1",
					},
					"field 2": {
						Name:         "field 2",
						PhysicalName: "field 2",
					},
					"field 3": {
						Name:         "field 3",
						PhysicalName: "field 3",
					},
					"new_name": {
						Name:         "new_name",
						PhysicalName: "old_name",
						Aliases:      []string{"new_name", "old_name"},
					},
				},
			},
		},
	}

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
		aggregateFamily, err := aggregationNodeToQueryFamily(dm, node)
		asserts.NoError(err)
		asserts.Equal("table", aggregateFamily.TableName,
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
		aggregateFamily, err := aggregationNodeToQueryFamily(dm, node)
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
		aggregateFamily, err := aggregationNodeToQueryFamily(dm, node)
		asserts.NoError(err)
		asserts.Equal("table", aggregateFamily.TableName)
		asserts.Equal(1, aggregateFamily.EqConditions.Size())
		asserts.True(aggregateFamily.EqConditions.Contains("field"))
		asserts.Equal(0, aggregateFamily.IneqConditions.Size())
		asserts.Equal(1, aggregateFamily.SelectOrOtherConditions.Size(),
			"SelectOrOtherConditions should contain field 0")
	})

	t.Run("with renamed field", func(t *testing.T) {
		asserts := assert.New(t)
		node := ast.Node{
			Function: ast.FUNC_AGGREGATOR,
			NamedChildren: map[string]ast.Node{
				"tableName": ast.NewNodeConstant("table"),
				"fieldName": ast.NewNodeConstant("new_name"),
				"filters": {
					Children: []ast.Node{
						{
							Function: ast.FUNC_FILTER,
							NamedChildren: map[string]ast.Node{
								"tableName": ast.NewNodeConstant("table"),
								"fieldName": ast.NewNodeConstant("new_name"),
								"operator":  ast.NewNodeConstant("="),
							},
						},
					},
				},
			},
		}
		aggregateFamily, err := aggregationNodeToQueryFamily(dm, node)
		asserts.NoError(err)
		asserts.Equal("table", aggregateFamily.TableName)
		asserts.Equal(1, aggregateFamily.EqConditions.Size())
		asserts.True(aggregateFamily.EqConditions.Contains("old_name"))
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
		aggregateFamily, err := aggregationNodeToQueryFamily(dm, node)
		asserts.NoError(err)
		asserts.Equal("table", aggregateFamily.TableName)
		asserts.Equal(1, aggregateFamily.EqConditions.Size())
		asserts.True(aggregateFamily.EqConditions.Contains("field"))
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
		aggregateFamily, err := aggregationNodeToQueryFamily(dm, node)
		asserts.NoError(err)
		asserts.Equal("table", aggregateFamily.TableName)
		asserts.Equal(1, aggregateFamily.EqConditions.Size())
		asserts.True(aggregateFamily.EqConditions.Contains("field 1"))
		asserts.Equal(1, aggregateFamily.IneqConditions.Size())
		asserts.True(aggregateFamily.IneqConditions.Contains("field 2"))
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
		_, err := aggregationNodeToQueryFamily(models.DataModel{}, node)
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
		aggregateFamily, err := aggregationNodeToQueryFamily(dm, node)
		asserts.NoError(err)
		asserts.Equal("table", aggregateFamily.TableName)
		asserts.Equal(2, aggregateFamily.EqConditions.Size(),
			"EqConditions should contain field 1 and field 2")
		asserts.True(aggregateFamily.EqConditions.Contains("field 1"), "EqConditions should contain field 1")
		asserts.True(aggregateFamily.EqConditions.Contains("field 2"), "EqConditions should contain field 2")
		asserts.Equal(1, aggregateFamily.IneqConditions.Size(),
			"IneqConditions should contain field 3")
		asserts.True(aggregateFamily.IneqConditions.Contains("field 3"),
			"IneqConditions should contain field 3")
		asserts.Equal(1, aggregateFamily.SelectOrOtherConditions.Size(),
			"SelectOrOtherConditions should contain 1 field")
		asserts.True(aggregateFamily.SelectOrOtherConditions.Contains("field 0"),
			"SelectOrOtherConditions should contain field 0")
	})
}

func TestAstNodeToQueryFamilies(t *testing.T) {
	ctx := makeTestContext()

	dm := models.DataModel{
		Tables: map[string]models.Table{
			"table": {
				Fields: map[string]models.Field{
					"field": {
						Name:         "field",
						PhysicalName: "field",
					},
					"field 0": {
						Name:         "field 0",
						PhysicalName: "field 0",
					},
					"field 1": {
						Name:         "field 1",
						PhysicalName: "field 1",
					},
					"field 2": {
						Name:         "field 2",
						PhysicalName: "field 2",
					},
					"field 3": {
						Name:         "field 3",
						PhysicalName: "field 3",
					},
				},
			},
		},
	}

	t.Run("empty node", func(t *testing.T) {
		asserts := assert.New(t)
		output, err := extractQueryFamiliesFromAst(ctx, dm, ast.Node{})
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
		output, err := extractQueryFamiliesFromAst(ctx, dm, ast)
		asserts.NoError(err)
		asserts.Equal(1, output.Size(), "There should be only 1 query family in the output set")
		expected := set.NewHashSet[models.AggregateQueryFamily](0)
		expected.Insert(models.AggregateQueryFamily{
			TableName:               "table",
			EqConditions:            set.From([]string{"field"}),
			IneqConditions:          set.New[string](0),
			SelectOrOtherConditions: set.From([]string{"field 0"}),
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
		output, err := extractQueryFamiliesFromAst(ctx, dm, ast)
		asserts.NoError(err)
		asserts.Equal(3, output.Size(), "There should be 2 query families in the output set")
		expected := set.NewHashSet[models.AggregateQueryFamily](0)
		expected.Insert(models.AggregateQueryFamily{
			TableName:               "table",
			EqConditions:            set.From([]string{"field"}),
			IneqConditions:          set.New[string](0),
			SelectOrOtherConditions: set.From([]string{"field 0"}),
		})
		expected.Insert(models.AggregateQueryFamily{
			TableName:               "table",
			EqConditions:            set.New[string](0),
			IneqConditions:          set.From([]string{"field"}),
			SelectOrOtherConditions: set.From([]string{"field 0"}),
		})
		expected.Insert(models.AggregateQueryFamily{
			TableName:      "table",
			EqConditions:   set.From([]string{"field 0"}),
			IneqConditions: set.New[string](0),
			SelectOrOtherConditions: set.From([]string{
				"field 2", "field 3",
			}),
		})
		asserts.True(output.EqualSet(expected), "The output set should contain the 2 query families")
		fmt.Println(expected)
		fmt.Println(output)
	})

	t.Run("with invalid child", func(t *testing.T) {
		asserts := assert.New(t)
		ast := ast.Node{
			Children: []ast.Node{
				{
					Function: ast.FUNC_AGGREGATOR,
					NamedChildren: map[string]ast.Node{
						"filters": {
							Children: []ast.Node{
								{
									Function: ast.FUNC_FILTER,
									NamedChildren: map[string]ast.Node{
										"tableName": ast.NewNodeConstant("table"),
										"operator":  ast.NewNodeConstant("="),
									},
								},
							},
						},
					},
				},
			},
		}
		out, err := extractQueryFamiliesFromAst(ctx, dm, ast)
		asserts.NoError(err)
		asserts.Equal(0, out.Size(), "There should be no query families in the output set")
	})
}

func Test_indexesToCreateFromScenarioIterations(t *testing.T) {
	ctx := makeTestContext()

	dm := models.DataModel{
		Tables: map[string]models.Table{
			"table": {
				Fields: map[string]models.Field{
					"field": {
						Name:         "field",
						PhysicalName: "field",
					},
					"field 0": {
						Name:         "field 0",
						PhysicalName: "field 0",
					},
					"field 1": {
						Name:         "field 1",
						PhysicalName: "field 1",
					},
					"field 2": {
						Name:         "field 2",
						PhysicalName: "field 2",
					},
					"field 3": {
						Name:         "field 3",
						PhysicalName: "field 3",
					},
				},
			},
		},
	}

	t.Run("empty input", func(t *testing.T) {
		asserts := assert.New(t)
		out, err := indexesToCreateFromScenarioIterations(ctx, dm, []models.ScenarioIteration{}, nil)
		asserts.NoError(err)
		asserts.Equal(0, len(out), "There should be no indexes to create")
	})

	t.Run("with one iteration and no existing indexes", func(t *testing.T) {
		asserts := assert.New(t)
		out, err := indexesToCreateFromScenarioIterations(ctx, dm, []models.ScenarioIteration{
			{
				TriggerConditionAstExpression: &ast.Node{
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
				},
			},
		}, nil)
		asserts.NoError(err)
		asserts.Equal(1, len(out), "There should be 1 index to create")
		asserts.Equal(models.ConcreteIndex{
			TableName: "table",
			Indexed:   []string{"field"},
			Included:  []string{"field 0"},
			Type:      models.IndexTypeAggregation,
		}, out[0])
	})

	t.Run("with one iteration, invalid aggregation (missing field name)", func(t *testing.T) {
		asserts := assert.New(t)
		out, err := indexesToCreateFromScenarioIterations(ctx, dm, []models.ScenarioIteration{
			{
				TriggerConditionAstExpression: &ast.Node{
					Function: ast.FUNC_AGGREGATOR,
					NamedChildren: map[string]ast.Node{
						"tableName": ast.NewNodeConstant("table"),
						"fieldName": ast.NewNodeConstant("field 0"),
						"filters": {
							Children: []ast.Node{
								{
									Function: ast.FUNC_FILTER,
									NamedChildren: map[string]ast.Node{
										// missing field name in filter
										"tableName": ast.NewNodeConstant("table"),
										"operator":  ast.NewNodeConstant("="),
									},
								},
							},
						},
					},
				},
			},
		}, nil)
		asserts.NoError(err)
		asserts.Equal(0, len(out), "There should be no indexes to create")
	})

	t.Run("scenario iteration without ASTs", func(t *testing.T) {
		asserts := assert.New(t)
		out, err := indexesToCreateFromScenarioIterations(ctx, dm, []models.ScenarioIteration{
			{
				TriggerConditionAstExpression: nil,
			},
		}, nil)
		asserts.NoError(err)
		asserts.Equal(0, len(out), "There should be no indexes to create")
	})
}
