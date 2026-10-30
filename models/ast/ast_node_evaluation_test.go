package ast

import (
	"errors"
	"testing"

	crdberrors "github.com/cockroachdb/errors"
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

// Regression test: cached evaluations used to be deep-copied with copystructure, which drops
// the unexported fields of wrapped errors and made calling Error() on them panic.
func TestNodeEvaluationCloneKeepsWrappedErrors(t *testing.T) {
	original := NodeEvaluation{
		Function: FUNC_AND,
		Errors:   []error{crdberrors.Wrap(ErrDivisionByZero, "wrapped")},
		Children: []NodeEvaluation{
			{Function: FUNC_DIVIDE, Errors: []error{crdberrors.WithStack(ErrRuntimeExpression)}},
		},
	}

	clone, err := original.Clone()
	require.NoError(t, err)

	require.NotPanics(t, func() {
		AdaptNodeEvaluationDto(clone)
	})
	assert.Equal(t, "wrapped: Division by zero: expression runtime error", clone.Errors[0].Error())
	assert.ErrorIs(t, clone.Errors[0], ErrDivisionByZero)
	assert.ErrorIs(t, clone.Children[0].Errors[0], ErrRuntimeExpression)
}
