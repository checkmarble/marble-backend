package evaluate

import (
	"context"
	"testing"
	"time"

	"github.com/checkmarble/marble-backend/models/ast"

	"github.com/stretchr/testify/assert"
)

func TestNotEqual_Evaluate_int(t *testing.T) {
	tests := []struct {
		name   string
		args   []any
		want   any
		errors []error
	}{
		{
			name:   "nominal",
			args:   []any{1, 1},
			want:   false,
			errors: []error{},
		},
		{
			name:   "big",
			args:   []any{999999999, 999999999},
			want:   false,
			errors: []error{},
		},
		{
			name:   "negative",
			args:   []any{-10, -10},
			want:   false,
			errors: []error{},
		},
		{
			name:   "not equal",
			args:   []any{1, 2},
			want:   true,
			errors: []error{},
		},
		{
			name:   "different types",
			args:   []any{1, "1"},
			want:   nil,
			errors: []error{ast.ErrArgumentInvalidType},
		},
		{
			name:   "Close floats",
			args:   []any{0.3, 0.2 + 0.1},
			want:   false,
			errors: []error{},
		},
		{
			name:   "'Regularly' close floats",
			args:   []any{0.3, 0.300001},
			want:   true,
			errors: []error{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, errs := NotEqual{}.Evaluate(context.TODO(), ast.Arguments{Args: tt.args})
			assert.Equal(t, len(tt.errors), len(errs))
			if len(errs) > 0 {
				assert.ErrorIs(t, errs[0], tt.errors[0])
			}
			assert.Equal(t, tt.want, r)
		})
	}
}

func TestNotEqual_Evaluate_float(t *testing.T) {
	tests := []struct {
		name   string
		args   []any
		want   any
		errors []error
	}{
		{
			name:   "nominal",
			args:   []any{22.3, 22.3},
			want:   false,
			errors: []error{},
		},
		{
			name:   "negative",
			args:   []any{-22.3, -22.3},
			want:   false,
			errors: []error{},
		},
		{
			name:   "not equal",
			args:   []any{22.3, 52.3},
			want:   true,
			errors: []error{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, errs := NotEqual{}.Evaluate(context.TODO(), ast.Arguments{Args: tt.args})
			assert.Equal(t, tt.errors, errs)
			assert.Equal(t, tt.want, r)
		})
	}
}

func TestNotEqual_Evaluate_string(t *testing.T) {
	tests := []struct {
		name   string
		args   []any
		want   any
		errors []error
	}{
		{
			name:   "nominal",
			args:   []any{"a", "b"},
			want:   true,
			errors: []error{},
		},
		{
			name:   "equal",
			args:   []any{"a", "a"},
			want:   false,
			errors: []error{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, errs := NotEqual{}.Evaluate(context.TODO(), ast.Arguments{Args: tt.args})
			assert.Equal(t, tt.errors, errs)
			assert.Equal(t, tt.want, r)
		})
	}
}

func TestNotEqual_Evaluate_bool(t *testing.T) {
	tests := []struct {
		name   string
		args   []any
		want   any
		errors []error
	}{
		{
			name:   "nominal",
			args:   []any{true, false},
			want:   true,
			errors: []error{},
		},
		{
			name:   "equal",
			args:   []any{true, true},
			want:   false,
			errors: []error{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, errs := NotEqual{}.Evaluate(context.TODO(), ast.Arguments{Args: tt.args})
			assert.Equal(t, tt.errors, errs)
			assert.Equal(t, tt.want, r)
		})
	}
}

func TestNotEqual_Evaluate_time(t *testing.T) {
	tests := []struct {
		name   string
		args   []any
		want   any
		errors []error
	}{
		{
			name:   "nominal",
			args:   []any{time.Date(2016, time.April, 25, 0, 0, 0, 0, time.UTC), time.Date(2016, time.April, 29, 0, 0, 0, 0, time.UTC)},
			want:   true,
			errors: []error{},
		},
		{
			name:   "equal",
			args:   []any{time.Date(2016, time.April, 29, 0, 0, 0, 0, time.UTC), time.Date(2016, time.April, 29, 0, 0, 0, 0, time.UTC)},
			want:   false,
			errors: []error{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, errs := NotEqual{}.Evaluate(context.TODO(), ast.Arguments{Args: tt.args})
			assert.Equal(t, tt.errors, errs)
			assert.Equal(t, tt.want, r)
		})
	}
}
