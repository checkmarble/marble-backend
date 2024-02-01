package models

import (
	"testing"

	"github.com/hashicorp/go-set/v2"
	"github.com/stretchr/testify/assert"
)

func TestExtractMinimalSetOfIdxFamilies(t *testing.T) {

	t.Run("Just one input family", func(t *testing.T) {
		asserts := assert.New(t)
		idxFamilies := set.HashSetFrom[IndexFamily]([]IndexFamily{
			{
				Fixed:  []FieldName{FieldName("a"), FieldName("b")},
				Flex:   set.From[FieldName]([]FieldName{FieldName("c")}),
				Last:   FieldName("d"),
				Others: set.From[FieldName]([]FieldName{FieldName("e"), FieldName("f")}),
			},
		})

		minimalSet := ExtractMinimalSetOfIdxFamilies(idxFamilies)
		asserts.Equal(1, minimalSet.Size(), "Keep just the one input family")
		asserts.True(minimalSet.Equal(idxFamilies), "The input and output sets are the same")
	})

	t.Run("2 non overlapping families", func(t *testing.T) {
		asserts := assert.New(t)
		idxFamilies := set.HashSetFrom[IndexFamily]([]IndexFamily{
			{
				Fixed:  []FieldName{},
				Flex:   set.From[FieldName]([]FieldName{FieldName("a")}),
				Last:   FieldName(""),
				Others: set.New[FieldName](0),
			},
			{
				Fixed:  []FieldName{},
				Flex:   set.From[FieldName]([]FieldName{FieldName("b")}),
				Last:   FieldName(""),
				Others: set.New[FieldName](0),
			},
		})

		minimalSet := ExtractMinimalSetOfIdxFamilies(idxFamilies)
		asserts.Equal(2, minimalSet.Size(), "Keep the two identical input families")
		asserts.True(minimalSet.Equal(idxFamilies), "The input and output sets are the same")
	})

	t.Run("2 overlapping families", func(t *testing.T) {
		asserts := assert.New(t)
		idxFamilies := set.HashSetFrom[IndexFamily]([]IndexFamily{
			{
				Fixed:  []FieldName{},
				Flex:   set.From[FieldName]([]FieldName{FieldName("a")}),
				Last:   FieldName(""),
				Others: set.New[FieldName](0),
			},
			{
				Fixed:  []FieldName{},
				Flex:   set.From[FieldName]([]FieldName{FieldName("a"), FieldName("b")}),
				Last:   FieldName(""),
				Others: set.New[FieldName](0),
			},
		})

		minimalSet := ExtractMinimalSetOfIdxFamilies(idxFamilies)
		asserts.Equal(2, minimalSet.Size(), "Keep the two identical input families")          // Pending actual implementation of the merge family method
		asserts.True(minimalSet.Equal(idxFamilies), "The input and output sets are the same") // Pending actual implementation of the merge family method
	})

}
