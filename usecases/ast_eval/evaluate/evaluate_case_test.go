package evaluate

import (
	"context"
	"testing"

	"github.com/checkmarble/marble-backend/models/ast"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCase(t *testing.T) {
	t.Run("matches and normalizes an integer value", func(t *testing.T) {
		result, errs := (Case{}).Evaluate(context.Background(), ast.Arguments{
			Args: []any{true, 42},
		})

		require.Empty(t, errs)
		assert.Equal(t, ast.SwitchCaseResult{Matched: true, Value: 42.0}, result)
	})

	t.Run("does not match false or nil predicates", func(t *testing.T) {
		for _, predicate := range []any{false, nil} {
			result, errs := (Case{}).Evaluate(context.Background(), ast.Arguments{
				Args: []any{predicate, 4.2},
			})

			require.Empty(t, errs)
			assert.Equal(t, ast.SwitchCaseResult{Matched: false, Value: 4.2}, result)
		}
	})

	t.Run("requires exactly two children", func(t *testing.T) {
		result, errs := (Case{}).Evaluate(context.Background(), ast.Arguments{
			Args: []any{true},
		})

		assert.Nil(t, result)
		require.Len(t, errs, 1)
		assert.ErrorIs(t, errs[0], ast.ErrWrongNumberOfArgument)
	})

	t.Run("rejects a non-boolean predicate at argument zero", func(t *testing.T) {
		result, errs := (Case{}).Evaluate(context.Background(), ast.Arguments{
			Args: []any{"true", 42},
		})

		assert.Nil(t, result)
		require.Len(t, errs, 1)
		assert.ErrorIs(t, errs[0], ast.ErrArgumentMustBeBool)
		assert.Equal(t, ast.NewArgumentError(0), extractArgumentError(t, errs[0]))
	})

	t.Run("rejects a non-numeric value at argument one", func(t *testing.T) {
		result, errs := (Case{}).Evaluate(context.Background(), ast.Arguments{
			Args: []any{true, "42"},
		})

		assert.Nil(t, result)
		require.Len(t, errs, 1)
		assert.ErrorIs(t, errs[0], ast.ErrArgumentMustBeIntOrFloat)
		assert.Equal(t, ast.NewArgumentError(1), extractArgumentError(t, errs[0]))
	})
}

func extractArgumentError(t *testing.T, err error) ast.ArgumentError {
	t.Helper()

	var argumentErr ast.ArgumentError
	require.ErrorAs(t, err, &argumentErr)
	return argumentErr
}
