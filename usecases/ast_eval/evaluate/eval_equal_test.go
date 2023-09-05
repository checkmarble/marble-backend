package evaluate

import (
	"fmt"
	"testing"

	"github.com/checkmarble/marble-backend/models/ast"

	"github.com/stretchr/testify/assert"
)

func TestEqual_Evaluate_int(t *testing.T) {
	tests := []struct {
		name   string
		args   []any
		want   any
		errors []error
	}{
		{
			name:   "nominal",
			args:   []any{1, 1},
			want:   true,
			errors: []error{},
		},
		{
			name:   "big",
			args:   []any{999999999, 999999999},
			want:   true,
			errors: []error{},
		},
		{
			name:   "negative",
			args:   []any{-10, -10},
			want:   true,
			errors: []error{},
		},
		{
			name:   "not equal",
			args:   []any{1, 2},
			want:   false,
			errors: []error{},
		},
		{
			name:   "different types",
			args:   []any{1, "1"},
			want:   nil,
			errors: []error{fmt.Errorf("all argments must be string, boolean, int or float")},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, errs := Equal{}.Evaluate(ast.Arguments{Args: tt.args})
			assert.Equal(t, tt.errors, errs)
			assert.Equal(t, tt.want, r)
		})
	}

}

func TestEqual_Evaluate_float(t *testing.T) {

	r, errs := Equal{}.Evaluate(ast.Arguments{Args: []any{22.3, 22.3}})
	assert.Empty(t, errs)
	assert.Equal(t, true, r)
}

func TestEqual_Evaluate_string(t *testing.T) {

	r, errs := Equal{}.Evaluate(ast.Arguments{Args: []any{"a", "a"}})
	assert.Empty(t, errs)
	assert.Equal(t, true, r)
}

func TestEqual_Evaluate_bool(t *testing.T) {

	r, errs := Equal{}.Evaluate(ast.Arguments{Args: []any{false, false}})
	assert.Empty(t, errs)
	assert.Equal(t, true, r)
}
