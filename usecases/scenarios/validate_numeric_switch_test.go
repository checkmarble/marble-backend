package scenarios

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/models/ast"
	"github.com/checkmarble/marble-backend/usecases/ast_eval"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateNumericSwitchesInAst(t *testing.T) {
	amount := payloadField("amount")
	score := payloadField("score")
	orgName := dbAccessField("transaction", "name", []string{"my_transaction", "myorganisation"})
	orgCountry := dbAccessField("transaction", "country", []string{"my_transaction", "myorganisation"})

	tests := []struct {
		name         string
		node         ast.Node
		wantErrCount int
		wantMessage  string
	}{
		{
			name: "one-variable numeric switch with non-decreasing thresholds",
			node: wrapSwitchInComparison(numericSwitch(
				oneVarNumericCase(amount, 100, 4),
				oneVarNumericCase(amount, 200, 6),
			)),
		},
		{
			name: "repeated thresholds are allowed",
			node: numericSwitch(
				oneVarNumericCase(amount, 100, 4),
				oneVarNumericCase(amount, 100, 5),
				oneVarNumericCase(amount, 200, 6),
			),
		},
		{
			name: "matrix with cartesian columns",
			node: numericSwitch(
				matrixNumericCase(amount, 100, orgName, "orga1", 4),
				matrixNumericCase(amount, 100, orgName, "orga2", 5),
				matrixNumericCase(amount, 100, orgName, "orga3", 6),
				matrixNumericCase(amount, 200, orgName, "orga1", 6),
				matrixNumericCase(amount, 200, orgName, "orga2", 7),
				matrixNumericCase(amount, 200, orgName, "orga3", 8),
			),
		},
		{
			name: "boolean equality switch is skipped",
			node: numericSwitch(
				booleanEqualityCase(payloadField("status"), "PEP", 25),
				booleanEqualityCase(payloadField("status"), "SANCTION", 50),
			),
		},
		{
			name: "two-string matrix is skipped",
			node: numericSwitch(
				twoStringMatrixCase(payloadField("status"), "PEP", orgName, "orga1", 4),
				twoStringMatrixCase(payloadField("status"), "PEP", orgName, "orga2", 5),
			),
		},
		{
			name: "scoring switch is skipped",
			node: ast.Node{Function: ast.FUNC_SWITCH}.
				AddChild(ast.Node{Function: ast.FUNC_SCORE_COMPUTATION}.
					AddChild(ast.NewNodeConstant(true)).
					AddNamedChild("modifier", ast.NewNodeConstant(10))),
		},
		{
			name: "decreasing thresholds",
			node: numericSwitch(
				oneVarNumericCase(amount, 200, 4),
				oneVarNumericCase(amount, 100, 6),
			),
			wantErrCount: 1,
			wantMessage:  "numeric switch thresholds must be non-decreasing",
		},
		{
			name: "mixed numeric and equality cases",
			node: numericSwitch(
				oneVarNumericCase(amount, 100, 4),
				booleanEqualityCase(payloadField("status"), "PEP", 25),
			),
			wantErrCount: 1,
			wantMessage:  "numeric switch mixes incompatible case predicates",
		},
		{
			name: "mixed one-variable and matrix cases",
			node: numericSwitch(
				oneVarNumericCase(amount, 100, 4),
				matrixNumericCase(amount, 200, orgName, "orga1", 6),
			),
			wantErrCount: 1,
			wantMessage:  "numeric switch mixes incompatible case predicates",
		},
		{
			name: "different first-dimension fields",
			node: numericSwitch(
				oneVarNumericCase(amount, 100, 4),
				oneVarNumericCase(score, 200, 6),
			),
			wantErrCount: 1,
			wantMessage:  "numeric switch cases must use the same field on the first dimension",
		},
		{
			name: "different second-dimension fields",
			node: numericSwitch(
				matrixNumericCase(amount, 100, orgName, "orga1", 4),
				matrixNumericCase(amount, 100, orgCountry, "FR", 5),
			),
			wantErrCount: 1,
			wantMessage:  "numeric switch matrix cases must use the same field on the second dimension",
		},
		{
			name: "non-constant threshold is not numeric-first-dimension",
			node: numericSwitch(
				ast.Node{Function: ast.FUNC_CASE}.
					AddChild(ast.Node{Function: ast.FUNC_LESS_OR_EQUAL}.
						AddChild(amount).
						AddChild(payloadField("limit"))).
					AddChild(ast.NewNodeConstant(4)),
			),
		},
		{
			name: "nested switch is still walked",
			node: wrapSwitchInComparison(numericSwitch(
				oneVarNumericCase(amount, 200, 4),
				oneVarNumericCase(amount, 100, 6),
			)),
			wantErrCount: 1,
			wantMessage:  "numeric switch thresholds must be non-decreasing",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := validateNumericSwitchesInAst(tt.node)
			require.Len(t, errs, tt.wantErrCount)
			for _, err := range errs {
				assert.Equal(t, models.NumericSwitchInvalid, err.Code)
				assert.ErrorIs(t, err.Error, models.BadParameterError)
			}
			if tt.wantMessage != "" {
				assert.ErrorContains(t, errs[0].Error, tt.wantMessage)
			}
		})
	}
}

func TestValidateNumericSwitchFromStoredJson(t *testing.T) {
	const formula = `{
		"name": ">",
		"children": [
			{
				"name": "Switch",
				"children": [
					{
						"name": "Case",
						"children": [
							{
								"name": "And",
								"children": [
									{
										"name": "<=",
										"children": [
											{"name": "Payload", "children": [{"constant": "amount"}]},
											{"constant": 100}
										]
									},
									{
										"name": "=",
										"children": [
											{
												"name": "DatabaseAccess",
												"named_children": {
													"fieldName": {"constant": "name"},
													"path": {"constant": ["my_transaction", "myorganisation"]},
													"tableName": {"constant": "transaction"}
												}
											},
											{"constant": "orga1"}
										]
									}
								]
							},
							{"constant": 4}
						]
					},
					{
						"name": "Case",
						"children": [
							{
								"name": "And",
								"children": [
									{
										"name": "<=",
										"children": [
											{"name": "Payload", "children": [{"constant": "amount"}]},
											{"constant": 200}
										]
									},
									{
										"name": "=",
										"children": [
											{
												"name": "DatabaseAccess",
												"named_children": {
													"fieldName": {"constant": "name"},
													"path": {"constant": ["my_transaction", "myorganisation"]},
													"tableName": {"constant": "transaction"}
												}
											},
											{"constant": "orga1"}
										]
									}
								]
							},
							{"constant": 6}
						]
					}
				],
				"named_children": {"fallback": {"constant": 10}}
			},
			{"constant": 5}
		]
	}`

	var nodeDto dto.NodeDto
	require.NoError(t, json.Unmarshal([]byte(formula), &nodeDto))
	node, err := dto.AdaptASTNode(nodeDto)
	require.NoError(t, err)
	assert.Empty(t, validateNumericSwitchesInAst(node))
}

func TestValidateScenarioAst_NumericSwitchStructuralRules(t *testing.T) {
	validator := ValidateScenarioAstImpl{
		AstValidator: staticAstValidator{
			environment: ast_eval.NewAstEvaluationEnvironment().WithoutOptimizations(),
		},
	}
	amount := payloadField("amount")
	formula := wrapSwitchInComparison(numericSwitch(
		oneVarNumericCase(amount, 200, 4),
		oneVarNumericCase(amount, 100, 6),
	))

	validation := validator.Validate(context.Background(), models.Scenario{}, &formula)

	require.Len(t, validation.Errors, 1)
	assert.Equal(t, models.NumericSwitchInvalid, validation.Errors[0].Code)
	assert.ErrorContains(t, validation.Errors[0].Error, "numeric switch thresholds must be non-decreasing")
}

func payloadField(name string) ast.Node {
	return ast.Node{Function: ast.FUNC_PAYLOAD}.AddChild(ast.NewNodeConstant(name))
}

func dbAccessField(table, field string, path []string) ast.Node {
	return ast.NewNodeDatabaseAccess(table, field, path)
}

func oneVarNumericCase(field ast.Node, threshold, value any) ast.Node {
	return ast.Node{Function: ast.FUNC_CASE}.
		AddChild(ast.Node{Function: ast.FUNC_LESS_OR_EQUAL}.
			AddChild(field).
			AddChild(ast.NewNodeConstant(threshold))).
		AddChild(ast.NewNodeConstant(value))
}

func matrixNumericCase(field ast.Node, threshold any, secondField ast.Node, secondValue string, value any) ast.Node {
	return ast.Node{Function: ast.FUNC_CASE}.
		AddChild(ast.Node{Function: ast.FUNC_AND}.
			AddChild(ast.Node{Function: ast.FUNC_LESS_OR_EQUAL}.
				AddChild(field).
				AddChild(ast.NewNodeConstant(threshold))).
			AddChild(ast.Node{Function: ast.FUNC_EQUAL}.
				AddChild(secondField).
				AddChild(ast.NewNodeConstant(secondValue)))).
		AddChild(ast.NewNodeConstant(value))
}

func booleanEqualityCase(field ast.Node, equalTo string, value any) ast.Node {
	return ast.Node{Function: ast.FUNC_CASE}.
		AddChild(ast.Node{Function: ast.FUNC_EQUAL}.
			AddChild(field).
			AddChild(ast.NewNodeConstant(equalTo))).
		AddChild(ast.NewNodeConstant(value))
}

func twoStringMatrixCase(fieldA ast.Node, valueA string, fieldB ast.Node, valueB string, value any) ast.Node {
	return ast.Node{Function: ast.FUNC_CASE}.
		AddChild(ast.Node{Function: ast.FUNC_AND}.
			AddChild(ast.Node{Function: ast.FUNC_EQUAL}.
				AddChild(fieldA).
				AddChild(ast.NewNodeConstant(valueA))).
			AddChild(ast.Node{Function: ast.FUNC_EQUAL}.
				AddChild(fieldB).
				AddChild(ast.NewNodeConstant(valueB)))).
		AddChild(ast.NewNodeConstant(value))
}

func numericSwitch(cases ...ast.Node) ast.Node {
	root := ast.Node{Function: ast.FUNC_SWITCH, Children: cases}
	return root.AddNamedChild("fallback", ast.NewNodeConstant(10))
}

func wrapSwitchInComparison(numericSwitch ast.Node) ast.Node {
	return ast.Node{Function: ast.FUNC_GREATER}.
		AddChild(numericSwitch).
		AddChild(ast.NewNodeConstant(5))
}
