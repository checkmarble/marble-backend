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
			name: "partial_token_set_ratio",
			args: []any{"old mc donald had a farm", "old mc donald may have had a farm"},
			algo: "partial_token_set_ratio",
			want: 100,
		},
		{
			name: "token_set_ratio",
			args: []any{"old mc donald had a farm", "old mc donald may have had a farm"},
			algo: "token_set_ratio",
			want: 100,
		},
		{
			name: "partial_ratio",
			args: []any{"old mc donald had a farm", "old mc donald may have had a farm"},
			algo: "partial_ratio",
			want: 75,
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
			algo: "token_set_ratio",
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
			name: "partial_token_set_ratio",
			args: []any{"old mc donald had a farm", []string{"old mc donald may have had a farm", "E I E I O"}},
			algo: "partial_token_set_ratio",
			want: 100,
		},
		{
			name: "token_set_ratio",
			args: []any{"old mc donald had a farm", []string{"E I E I O"}},
			algo: "token_set_ratio",
			want: 21,
		},
		{
			name: "partial_ratio",
			args: []any{"old mc donald had a farm", []string{
				"Lorem ipsum dolor sit amet, consectetur adipiscing elit",
				"sed do eiusmod tempor incididunt ut labore et dolore magna aliqua",
			}},
			algo: "partial_ratio",
			want: 46,
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

func TestCleanseString(t *testing.T) {
	tests := []struct {
		name string
		args string
		want string
	}{
		{
			name: "cleanse string",
			args: "old mc donald had a farm",
			want: "old mc donald had a farm",
		},
		{
			name: "cleanse string with special characters",
			args: "old mc donald had a farm!@#$%^&*()",
			want: "old mc donald had a farm",
		},
		{
			name: "cleanse string with accents",
			args: "il était une fois une belle théière à ma sœur et ça c'est beau",
			want: "il etait une fois une belle theiere a ma sœur et ca c est beau",
		},
		{
			name: "various accents with upper case",
			args: "AÉÇÀÈÙÎÏ",
			want: "aecaeuii",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, cleanseString(tt.args))
		})
	}
}
