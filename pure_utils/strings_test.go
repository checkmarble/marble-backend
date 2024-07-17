package pure_utils

import (
	"fmt"
	"testing"

	"github.com/adrg/strutil"
	"github.com/adrg/strutil/metrics"
	"github.com/stretchr/testify/assert"
)

func TestBagOfWordsSimilarity(t *testing.T) {
	examples := []struct {
		s1       string
		s2       string
		expected int
	}{
		{"", "", 100},
		{"teatime", "tea time", 93},
		{"the dog was walking on the sidewalk", "the dog was walking on the side walk", 98},
		{"the dog was walking on the sidewalk", "the d og as walkin' on the side alk", 72},
		{"Mr Mrs John Jane OR Doe Smith	", "John Doe", 100},
		{"ça, c'est une théière", "la theier a une typo", 65},
	}

	for _, example := range examples {
		t.Run(example.s1+" vs "+example.s2, func(t *testing.T) {
			result := BagOfWordsSimilarity(example.s1, example.s2)
			assert.Equal(t, example.expected, result)
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
		{
			name: "theiere",
			args: "ça, c'est une théière",
			want: "ca  c est une theiere",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, cleanseString(tt.args))
		})
	}
}

func TestSimilarityRatio(t *testing.T) {
	tests := []struct {
		name     string
		s1       string
		s2       string
		expected float64
	}{
		{"empty strings", "", "", 100},
		{"same strings", "hello", "hello", 100},
		{"completely different strings", "hello", "aaaaa", 0},
		{
			"different strings with accents",
			"the dog was walking on the sidewalk", "the d og as walkin' on the side alk", 91,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := similarityRatio(tt.s1, tt.s2)
			assert.InDelta(t, tt.expected, result, 0.01)
		})
	}
}

func TestLehvenstein2(t *testing.T) {
	tests := []struct {
		name     string
		s1       string
		s2       string
		expected int
	}{
		{"different strings", "une c ca est theiere", "une a la theier typo", 14},
		{"all different strings", "world", "maiji", 10},
		{"from other test", "old mc donald had a farm", "old mc donald may have had a farm", 9},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lev := metrics.NewLevenshtein()
			lev.InsertCost = 1
			lev.ReplaceCost = 2
			lev.DeleteCost = 1

			similarity := strutil.Similarity("make", "Cake", lev)
			fmt.Printf("%.2f\n", similarity)

			result := lev.Distance(tt.s1, tt.s2)
			assert.Equal(t, tt.expected, result)
		})
	}
}
