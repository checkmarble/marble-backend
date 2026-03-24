package evaluate

import (
	"testing"

	"github.com/checkmarble/marble-backend/models/ast"
	"github.com/cockroachdb/errors"
	"github.com/stretchr/testify/assert"
)

func TestScoreComputationNoModifier(t *testing.T) {
	c := ScoreComputation{}

	args := ast.Arguments{
		Args:      []any{nil},
		NamedArgs: map[string]any{},
	}

	_, err := c.Evaluate(t.Context(), args)

	assert.Error(t, errors.Join(err...))
	assert.Contains(t, errors.Join(err...).Error(), "modifier not found")
}

func TestScoreComputationSetFloor(t *testing.T) {
	c := ScoreComputation{}

	args := ast.Arguments{
		Args:      []any{true},
		NamedArgs: map[string]any{"modifier": 42, "floor": 4},
	}

	out, err := c.Evaluate(t.Context(), args)
	result := out.(ast.ScoreComputationResult)

	assert.NoError(t, errors.Join(err...))
	assert.Equal(t, 42, result.Modifier)
	assert.Equal(t, 4, result.Floor)
}

func TestScoreComputationNilValue(t *testing.T) {
	c := ScoreComputation{}

	args := ast.Arguments{
		Args: []any{nil},
		NamedArgs: map[string]any{
			"modifier": 1,
		},
	}

	out, err := c.Evaluate(t.Context(), args)
	result := out.(ast.ScoreComputationResult)

	assert.NoError(t, errors.Join(err...))
	assert.Equal(t, 0, result.Modifier)
	assert.Equal(t, 0, result.Floor)
	assert.False(t, result.Triggered)
}

func TestScoreComputationNominal(t *testing.T) {
	c := ScoreComputation{}

	args := ast.Arguments{
		Args: []any{true},
		NamedArgs: map[string]any{
			"modifier": 1,
		},
	}

	out, err := c.Evaluate(t.Context(), args)
	result := out.(ast.ScoreComputationResult)

	assert.NoError(t, errors.Join(err...))
	assert.True(t, result.Triggered)
	assert.Equal(t, 1, result.Modifier)
	assert.Equal(t, 0, result.Floor)

	args = ast.Arguments{
		Args: []any{false},
		NamedArgs: map[string]any{
			"modifier": 2,
		},
	}

	out, err = c.Evaluate(t.Context(), args)
	result = out.(ast.ScoreComputationResult)

	assert.NoError(t, errors.Join(err...))
	assert.False(t, result.Triggered)
	assert.Equal(t, 0, result.Modifier)
	assert.Equal(t, 0, result.Floor)
}
