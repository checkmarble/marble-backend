package api

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseGraphDegrees(t *testing.T) {
	tests := []struct {
		name     string
		degrees  string
		depth    string
		expected int
	}{
		{name: "absent lets the usecase default", expected: 0},
		{name: "zero lets the usecase default", degrees: "0", expected: 0},
		{name: "negative lets the usecase default", degrees: "-3", expected: 0},
		{name: "unparseable lets the usecase default", degrees: "many", expected: 0},
		{name: "explicit value", degrees: "3", expected: 3},
		{name: "an over-large value is clamped by the usecase, not here", degrees: "99", expected: 99},
		{name: "depth is accepted as an alias", depth: "3", expected: 3},
		{name: "degrees wins over the alias", degrees: "1", depth: "4", expected: 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, parseGraphDegrees(tt.degrees, tt.depth))
		})
	}
}

func TestParseGraphEndTypes(t *testing.T) {
	tests := []struct {
		name     string
		raw      string
		expected []string
	}{
		{name: "empty defaults to the party tables", raw: ""},
		{name: "blank defaults to the party tables", raw: "  ,, "},
		{name: "single value", raw: "users", expected: []string{"users"}},
		{name: "trims and drops blanks", raw: " users , , companies ", expected: []string{"users", "companies"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, parseGraphEndTypes(tt.raw))
		})
	}
}
