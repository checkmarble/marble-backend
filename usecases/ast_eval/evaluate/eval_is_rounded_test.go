package evaluate

import (
	"context"
	"testing"

	"github.com/checkmarble/marble-backend/models/ast"
	"github.com/cockroachdb/errors"
	"github.com/stretchr/testify/assert"
)

func TestIsRounded(t *testing.T) {
	tests := []struct {
		name   string
		args   map[string]any
		want   any
		errors []error
	}{
		{
			name:   "with threshold not a power of 10",
			args:   map[string]any{"value": 1000, "threshold": 5},
			errors: []error{errors.New("Threshold argument must be a power of 10, got 5")},
		},
		{
			name:   "with a value which is not a float or an int",
			args:   map[string]any{"value": "hello world", "threshold": 10},
			errors: []error{ast.ErrArgumentMustBeIntOrFloat},
		},
		{
			name: "with a value which is not an int",
			args: map[string]any{"value": 100.1, "threshold": 10},
			want: false,
		},
		{
			name: "with value: 100, threshold: 10",
			args: map[string]any{"value": 100, "threshold": 10},
			want: true,
		},
		{
			name: "with value: 1000, threshold: 100",
			args: map[string]any{"value": 1000, "threshold": 100},
			want: true,
		},
		{
			name: "with value: 10, threshold: 10",
			args: map[string]any{"value": 10, "threshold": 10},
			want: true,
		},
		{
			name: "with value: 1000000, threshold: 10000",
			args: map[string]any{"value": 1000000, "threshold": 10000},
			want: true,
		},
		{
			name: "with value: 101, threshold: 10",
			args: map[string]any{"value": 101, "threshold": 10},
			want: false,
		},
		{
			name: "with value: 1005, threshold: 100",
			args: map[string]any{"value": 1005, "threshold": 100},
			want: false,
		},
		{
			name: "with value: 999, threshold: 1000",
			args: map[string]any{"value": 999, "threshold": 1000},
			want: false,
		},
		{
			name: "with value: 12345, threshold: 100",
			args: map[string]any{"value": 12345, "threshold": 100},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, errs := IsRounded{}.Evaluate(context.TODO(), ast.Arguments{
				Args:      []any{},
				NamedArgs: tt.args,
			})
			assert.Equal(t, len(tt.errors), len(errs))
			if len(errs) > 0 {
				assert.ErrorContains(t, errs[0], tt.errors[0].Error())
			}
			assert.Equal(t, tt.want, r)
		})
	}
}
