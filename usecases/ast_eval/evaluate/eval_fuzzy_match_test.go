package evaluate

import (
	"context"
	"testing"

	"github.com/checkmarble/marble-backend/models/ast"
	"github.com/cockroachdb/errors"
	"github.com/stretchr/testify/assert"
)

func TestFuzzyMatch(t *testing.T) {
	tests := []struct {
		name   string
		args   []any
		algo   string
		want   any
		errors []error
	}{
		{
			name: "bag_of_words_similarity",
			args: []any{"old mc donald had a farm", "old mc donald may have had a farm"},
			algo: "bag_of_words_similarity",
			want: 100,
		},
		{
			name:   "error algo",
			args:   []any{"old mc donald had a farm", "old mc donald may have had a farm"},
			algo:   "unknown",
			errors: []error{errors.New("Unknown algorithm: unknown")},
		},
		{
			name: "with accents",
			args: []any{"ça, c'est une théière", "la theier a une typo"},
			algo: "bag_of_words_similarity",
			want: 65,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, errs := FuzzyMatch{}.Evaluate(context.TODO(), ast.Arguments{
				Args:      tt.args,
				NamedArgs: map[string]any{"algorithm": tt.algo},
			})
			assert.Equal(t, len(tt.errors), len(errs))
			if len(errs) > 0 {
				assert.ErrorContains(t, errs[0], tt.errors[0].Error())
			}
			assert.Equal(t, tt.want, r)
		})
	}
}

func TestFuzzyMatchAnyOf(t *testing.T) {
	tests := []struct {
		name   string
		args   []any
		algo   string
		want   any
		errors []error
	}{
		{
			name: "bag_of_words_similarity",
			args: []any{"old mc donald had a farm", []string{"E I E I O"}},
			algo: "bag_of_words_similarity",
			want: 21,
		},
		{
			name: "ratio",
			args: []any{"old mc donald had a farm", []string{
				"Lorem ipsum dolor sit amet, consectetur adipiscing elit",
				"sed do eiusmod tempor incididunt ut labore et dolore magna aliqua",
			}},
			algo: "ratio",
			want: 31,
		},
		{
			name:   "error algo",
			args:   []any{"old mc donald had a farm", "old mc donald may have had a farm"},
			algo:   "unknown",
			errors: []error{errors.New("arguments must be a list")},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, errs := FuzzyMatchAnyOf{}.Evaluate(context.TODO(), ast.Arguments{
				Args:      tt.args,
				NamedArgs: map[string]any{"algorithm": tt.algo},
			})
			assert.Equal(t, len(tt.errors), len(errs))
			if len(errs) > 0 {
				assert.ErrorContains(t, errs[0], tt.errors[0].Error())
			}
			assert.Equal(t, tt.want, r)
		})
	}
}
