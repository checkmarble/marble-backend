package ast

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNodeEvaluationClone(t *testing.T) {
	list := []any{"a", "b"}

	original := NodeEvaluation{
		Function:    FUNC_AND,
		ReturnValue: true,
		Errors:      []error{errors.New("boom")},
		Children: []NodeEvaluation{
			{Function: FUNC_LIST, ReturnValue: list},
		},
		NamedChildren: map[string]NodeEvaluation{
			"child": {Function: FUNC_PAYLOAD, ReturnValue: 42},
		},
	}

	clone, err := original.Clone()
	require.NoError(t, err)
	assert.Equal(t, original, clone)

	clone.SetCached()
	clone.Errors[0] = errors.New("other")
	clone.Children[0].ReturnValue.([]any)[0] = "z"
	clone.NamedChildren["child"] = NodeEvaluation{ReturnValue: 0}

	assert.False(t, original.EvaluationPlan.Cached)
	assert.False(t, original.Children[0].EvaluationPlan.Cached)
	assert.Equal(t, "boom", original.Errors[0].Error())
	assert.Equal(t, []any{"a", "b"}, original.Children[0].ReturnValue)
	assert.Equal(t, 42, original.NamedChildren["child"].ReturnValue)
}
