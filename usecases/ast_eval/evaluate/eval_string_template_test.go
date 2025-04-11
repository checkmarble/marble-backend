package evaluate

import (
	"context"
	"testing"

	"github.com/checkmarble/marble-backend/models/ast"
	"github.com/stretchr/testify/assert"
)

func TestStringTemplate_Evaluate(t *testing.T) {
	tests := []struct {
		name      string
		args      ast.Arguments
		want      any
		wantError bool
	}{
		{
			name: "valid template with string variable",
			args: ast.Arguments{
				Args: []any{"Hello %name%"},
				NamedArgs: map[string]any{
					"name": "World",
				},
			},
			want:      "Hello World",
			wantError: false,
		},
		{
			name: "valid template with number variables",
			args: ast.Arguments{
				Args: []any{"Count: %count%, Price: %price%"},
				NamedArgs: map[string]any{
					"count": 42,
					"price": 19.99,
				},
			},
			want:      "Count: 42, Price: 19.99",
			wantError: false,
		},
		{
			name: "missing variable replaced with {}",
			args: ast.Arguments{
				Args:      []any{"Hello %name%"},
				NamedArgs: map[string]any{},
			},
			want:      "Hello {}",
			wantError: false,
		},
		{
			name: "nil template",
			args: ast.Arguments{
				Args: []any{nil},
			},
			want:      nil,
			wantError: true,
		},
		{
			name: "empty template",
			args: ast.Arguments{
				Args: []any{""},
			},
			want:      nil,
			wantError: true,
		},
		{
			name: "invalid variable type",
			args: ast.Arguments{
				Args: []any{"Value: %val%"},
				NamedArgs: map[string]any{
					"val": []string{"invalid"},
				},
			},
			want:      nil,
			wantError: true,
		},
	}

	stringTemplate := StringTemplate{}
	ctx := context.Background()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, errs := stringTemplate.Evaluate(ctx, tt.args)
			if tt.wantError {
				assert.NotNil(t, errs)
			} else {
				assert.Nil(t, errs)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}
