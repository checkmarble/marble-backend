package models

import (
	"testing"

	"github.com/checkmarble/marble-backend/models/ast"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTransientDecisionRoundTrip(t *testing.T) {
	decisionId := uuid.New()
	orgId := uuid.New()
	formula := ast.Node{Function: ast.FUNC_EQUAL, Children: []ast.Node{{Constant: true}, {Constant: true}}}

	bundle := DecisionBundle{
		Decision: DecisionWithRuleExecutions{
			Decision: Decision{
				DecisionId:     decisionId,
				OrganizationId: orgId,
				Outcome:        BlockAndReview,
				Score:          42,
				ClientObject:   ClientObject{TableName: "transactions", Data: map[string]any{"object_id": "obj"}},
			},
			RuleExecutions: []RuleExecution{
				{
					Id:                  "re1",
					Outcome:             "hit",
					Result:              true,
					ResultScoreModifier: 42,
					Evaluation:          &ast.NodeEvaluationDto{ReturnValue: ast.ReturnValueDto{Value: true}},
					Rule: Rule{
						Id:                   "rule1",
						StableRuleId:         "stable1",
						Name:                 "Rule 1",
						Description:          "Description",
						ScoreModifier:        42,
						FormulaAstExpression: &formula,
					},
				},
			},
		},
		Scenario:        Scenario{Id: "scenario", Name: "Scenario", TriggerObjectType: "transactions"},
		Execution:       ScenarioExecution{RuleExecutions: make([]RuleExecution, 1)},
		AnalyticsFields: map[string]any{"field": "value"},
		ObjectId:        "obj",
	}

	payload, err := NewTransientDecision(bundle).Encode()
	require.NoError(t, err)

	td, err := DecodeTransientDecision(payload)
	require.NoError(t, err)

	out := td.ToBundle()

	assert.Equal(t, decisionId, out.Decision.DecisionId)
	assert.Equal(t, orgId, out.Decision.OrganizationId)
	assert.Equal(t, BlockAndReview, out.Decision.Outcome)
	assert.Equal(t, 42, out.Decision.Score)
	assert.Equal(t, "transactions", out.Scenario.TriggerObjectType)
	assert.Equal(t, "obj", out.ObjectId)
	assert.Equal(t, map[string]any{"field": "value"}, out.AnalyticsFields)

	require.Len(t, out.Decision.RuleExecutions, 1)

	re := out.Decision.RuleExecutions[0]

	assert.Equal(t, "hit", re.Outcome)
	assert.True(t, re.Result)
	assert.Equal(t, 42, re.ResultScoreModifier)
	assert.Equal(t, true, re.Evaluation.ReturnValue.Value)
	assert.Equal(t, "rule1", re.Rule.Id)
	assert.Equal(t, "stable1", re.Rule.StableRuleId)
	assert.Equal(t, "Rule 1", re.Rule.Name)
	assert.Equal(t, "Description", re.Rule.Description)
	assert.Nil(t, re.Rule.FormulaAstExpression)

	// The original bundle must not be mutated.
	assert.NotNil(t, bundle.Decision.RuleExecutions[0].Rule.FormulaAstExpression)
}

func TestDecodeTransientDecisionInvalidPayload(t *testing.T) {
	_, err := DecodeTransientDecision([]byte("not zstd"))
	assert.Error(t, err)
}
