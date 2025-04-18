package evaluate

import (
	"context"
	"testing"

	"github.com/checkmarble/marble-backend/models/ast"
	"github.com/stretchr/testify/assert"
)

func TestFuzzyMatchOptionsEvaluator_Evaluate(t *testing.T) {
	evaluator := FuzzyMatchOptionsEvaluator{}
	ctx := context.Background()

	tests := []struct {
		name     string
		args     ast.Arguments
		want     ast.FuzzyMatchOptions
		wantErrs bool
	}{
		{
			name: "valid arguments",
			args: ast.Arguments{
				NamedArgs: map[string]any{
					"algorithm": "bag_of_words_similarity_db",
					"threshold": 75,
					"value":     "test string",
				},
			},
			want: ast.FuzzyMatchOptions{
				Algorithm: "bag_of_words_similarity_db",
				Threshold: 0.75,
				Value:     "test string",
			},
			wantErrs: false,
		},
		{
			name: "invalid algorithm",
			args: ast.Arguments{
				NamedArgs: map[string]any{
					"algorithm": "invalid_algorithm",
					"threshold": 75,
					"value":     "test string",
				},
			},
			wantErrs: true,
		},
		{
			name: "threshold too high",
			args: ast.Arguments{
				NamedArgs: map[string]any{
					"algorithm": "bag_of_words_similarity_db",
					"threshold": 101,
					"value":     "test string",
				},
			},
			wantErrs: true,
		},
		{
			name: "threshold too low",
			args: ast.Arguments{
				NamedArgs: map[string]any{
					"algorithm": "bag_of_words_similarity_db",
					"threshold": -1,
					"value":     "test string",
				},
			},
			wantErrs: true,
		},
		{
			name: "missing algorithm",
			args: ast.Arguments{
				NamedArgs: map[string]any{
					"threshold": 75,
					"value":     "test string",
				},
			},
			wantErrs: true,
		},
		{
			name: "missing threshold",
			args: ast.Arguments{
				NamedArgs: map[string]any{
					"algorithm": "bag_of_words_similarity_db",
					"value":     "test string",
				},
			},
			wantErrs: true,
		},
		{
			name: "missing value",
			args: ast.Arguments{
				NamedArgs: map[string]any{
					"algorithm": "bag_of_words_similarity_db",
					"threshold": 75,
				},
			},
			wantErrs: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, errs := evaluator.Evaluate(ctx, tt.args)

			if tt.wantErrs {
				assert.NotEmpty(t, errs)
				return
			}

			assert.Empty(t, errs)
			assert.Equal(t, tt.want, got)
		})
	}
}
