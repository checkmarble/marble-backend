package evaluate

import (
	"fmt"
	"testing"
	"time"

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
			errors: []error{fmt.Errorf("all arguments must be string, boolean, time, int or float %w", ast.ErrArgumentInvalidType)},
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

func TestEqual_Evaluate_time(t *testing.T) {

	r, errs := Equal{}.Evaluate(ast.Arguments{Args: []any{time.Date(2016, time.April, 29, 0, 0, 0, 0, time.UTC), time.Date(2016, time.April, 29, 0, 0, 0, 0, time.UTC)}})
	assert.Empty(t, errs)
	assert.Equal(t, true, r)
}
