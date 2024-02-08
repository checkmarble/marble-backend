package models

import (
	"fmt"
	"testing"

	"github.com/hashicorp/go-set/v2"
	"github.com/stretchr/testify/assert"
)

func TestExtractMinimalSetOfIdxFamilies(t *testing.T) {

	t.Run("Just one input family", func(t *testing.T) {
		asserts := assert.New(t)
		idxFamilies := set.HashSetFrom[IndexFamily]([]IndexFamily{
			{
				Fixed:    []FieldName{FieldName("a"), FieldName("b")},
				Flex:     set.From[FieldName]([]FieldName{FieldName("c")}),
				Last:     FieldName("d"),
				Included: set.From[FieldName]([]FieldName{FieldName("e"), FieldName("f")}),
			},
		})

		minimalSet := extractMinimalSetOfIdxFamilies(idxFamilies)
		asserts.Equal(1, minimalSet.Size(), "Keep just the one input family")
		asserts.True(minimalSet.Equal(idxFamilies), "The input and output sets are the same")
	})

	t.Run("2 non overlapping families", func(t *testing.T) {
		asserts := assert.New(t)
		idxFamilies := set.HashSetFrom[IndexFamily]([]IndexFamily{
			{
				Fixed:    []FieldName{},
				Flex:     set.From[FieldName]([]FieldName{FieldName("a")}),
				Last:     FieldName(""),
				Included: set.New[FieldName](0),
			},
			{
				Fixed:    []FieldName{},
				Flex:     set.From[FieldName]([]FieldName{FieldName("b")}),
				Last:     FieldName(""),
				Included: set.New[FieldName](0),
			},
		})

		minimalSet := extractMinimalSetOfIdxFamilies(idxFamilies)
		asserts.Equal(2, minimalSet.Size(), "Keep the two identical input families")
		asserts.True(minimalSet.Equal(idxFamilies), "The input and output sets are the same")
	})

	t.Run("2 overlapping families", func(t *testing.T) {
		asserts := assert.New(t)
		expected := set.HashSetFrom[IndexFamily]([]IndexFamily{{
			Fixed:    []FieldName{},
			Flex:     set.From[FieldName]([]FieldName{FieldName("a"), FieldName("b")}),
			Last:     FieldName(""),
			Included: set.New[FieldName](0),
		}})
		idxFamilies := set.HashSetFrom[IndexFamily]([]IndexFamily{
			{
				Fixed:    []FieldName{},
				Flex:     set.From[FieldName]([]FieldName{FieldName("a")}),
				Last:     FieldName(""),
				Included: set.New[FieldName](0),
			},
		})

		minimalSet := extractMinimalSetOfIdxFamilies(idxFamilies.Union(expected).(*set.HashSet[IndexFamily, string]))
		asserts.Equal(1, minimalSet.Size(), "Keep just one idx family")
		asserts.True(minimalSet.Equal(expected), "Keep the second input idx families")
	})

}

func TestRefineIdxFamiliesShortHasNoFixed(t *testing.T) {
	asserts := assert.New(t)

	t.Run("Simple case 1: no overlap", func(t *testing.T) {
		fam1 := NewIndexFamily()
		fam2 := NewIndexFamily()
		fam1.Flex.Insert(FieldName("a"))
		fam2.Flex.Insert(FieldName("b"))

		_, found := refineIdxFamilies(fam1, fam2)
		asserts.False(found, "No overlap expected")
	})

	t.Run("Simple case 1: identical", func(t *testing.T) {
		fam1 := NewIndexFamily()
		fam2 := NewIndexFamily()
		fam1.Flex.Insert(FieldName("a"))
		fam2.Flex.Insert(FieldName("a"))

		output, found := refineIdxFamilies(fam1, fam2)
		asserts.True(found, "Expect identical output")
		asserts.True(output.Equal(fam1), "Expect identical output")
	})

	t.Run("Simple case 2: simple overlap", func(t *testing.T) {
		fam1 := NewIndexFamily()
		fam2 := NewIndexFamily()
		fam1.Flex.Insert(FieldName("a"))
		fam2.Flex.InsertSlice([]FieldName{"a", "b"})

		output, found := refineIdxFamilies(fam1, fam2)
		asserts.True(found, "Expect identical output")
		asserts.True(output.Equal(fam2), "Expect to keep the second one")
	})

	t.Run("Simple case 2: simple overlap 3", func(t *testing.T) {
		fam1 := NewIndexFamily()
		fam2 := NewIndexFamily()
		fam1.Flex.InsertSlice([]FieldName{"a", "c"})
		fam2.Flex.InsertSlice([]FieldName{"a", "b", "c"})

		output, found := refineIdxFamilies(fam1, fam2)
		asserts.True(found, "Expect identical output")
		asserts.True(output.Equal(fam2), "Expect to keep the second one")
	})

	t.Run("Disjoint: no way to combine flex that are disjoint", func(t *testing.T) {
		fam1 := NewIndexFamily()
		fam2 := NewIndexFamily()
		fam1.Flex.InsertSlice([]FieldName{"a", "c"})
		fam2.Flex.InsertSlice([]FieldName{"a", "b"})

		_, found := refineIdxFamilies(fam1, fam2)
		asserts.False(found, "No solution to merge them")
	})

	t.Run("With fixed: 1", func(t *testing.T) {
		fam1 := NewIndexFamily()
		fam2 := NewIndexFamily()
		fam1.Fixed = []FieldName{"a", "b"}
		fam1.Flex.InsertSlice([]FieldName{"c", "d"})
		fam1.setLast("e")
		fam2.Fixed = []FieldName{"a", "b"}

		output, found := refineIdxFamilies(fam1, fam2)
		asserts.True(found, "There is a trivial solution to merge them")
		asserts.True(output.Equal(fam1), "Expect to keep the first one")
	})

	t.Run("With fixed: 2", func(t *testing.T) {
		fam1 := NewIndexFamily()
		fam2 := NewIndexFamily()
		fam1.Fixed = []FieldName{"a", "b"}
		fam1.Flex.InsertSlice([]FieldName{"c", "d"})
		fam1.setLast("e")
		fam2.Fixed = []FieldName{"a", "b"}
		fam2.setLast("c")

		output, found := refineIdxFamilies(fam1, fam2)
		asserts.True(found, "There is a solution to merge them")
		expected := NewIndexFamily()
		expected.Fixed = []FieldName{"a", "b", "c"}
		expected.Flex.InsertSlice([]FieldName{"d"})
		expected.setLast("e")
		asserts.True(output.Equal(expected), "Expect to keep the first one")
	})

	t.Run("With fixed incompatible", func(t *testing.T) {
		fam1 := NewIndexFamily()
		fam2 := NewIndexFamily()
		fam1.Fixed = []FieldName{"a", "b"}
		fam1.Flex.InsertSlice([]FieldName{"c", "d"})
		fam1.setLast("e")
		fam2.Fixed = []FieldName{"a", "c"}
		fam2.setLast("c")

		_, found := refineIdxFamilies(fam1, fam2)
		asserts.False(found, "No solution to merge them")
	})

	t.Run("With fixed incompatible", func(t *testing.T) {
		fam1 := NewIndexFamily()
		fam2 := NewIndexFamily()
		fam1.Fixed = []FieldName{"a", "b"}
		fam1.Flex.InsertSlice([]FieldName{"c", "d"})
		fam1.setLast("e")
		fam2.Fixed = []FieldName{"a", "c"}
		fam2.setLast("c")

		_, found := refineIdxFamilies(fam1, fam2)
		asserts.False(found, "No solution to merge them")
	})

	t.Run("1", func(t *testing.T) {
		A := NewIndexFamily()
		B := NewIndexFamily()
		A.Fixed = []FieldName{"a", "b"}
		A.Flex.InsertSlice([]FieldName{"c", "d"})
		A.setLast("e")
		B.Flex.InsertSlice([]FieldName{"a", "c", "f"})

		_, found := refineIdxFamilies(A, B)
		asserts.False(found, "No solution to merge them")
	})

	t.Run("2", func(t *testing.T) {
		A := NewIndexFamily()
		B := NewIndexFamily()
		A.Flex.InsertSlice([]FieldName{"a", "b", "c", "d"})
		A.setLast("e")
		B.Flex.InsertSlice([]FieldName{"a", "c", "f"})

		_, found := refineIdxFamilies(A, B)
		asserts.False(found, "No solution to merge them")
	})

	t.Run("3", func(t *testing.T) {
		A := NewIndexFamily()
		B := NewIndexFamily()
		A.Flex.InsertSlice([]FieldName{"a", "b", "c", "d"})
		A.setLast("e")
		B.Flex.InsertSlice([]FieldName{"a", "b", "c"})
		B.setLast("d")

		output, found := refineIdxFamilies(A, B)
		asserts.True(found, "There is a solution to merge them")
		expected := NewIndexFamily()
		expected.Fixed = []FieldName{"a", "b", "c", "d", "e"}
		asserts.True(output.Equal(expected), "Expect to keep the first one")
	})

	t.Run("4", func(t *testing.T) {
		A := NewIndexFamily()
		B := NewIndexFamily()
		A.Flex.InsertSlice([]FieldName{"a", "b"})
		A.setLast("c")
		B.Flex.InsertSlice([]FieldName{"a", "c"})
		B.setLast("b")

		_, found := refineIdxFamilies(A, B)
		asserts.False(found, "There is no solution to merge them")

	})

	t.Run("5", func(t *testing.T) {
		A := NewIndexFamily()
		B := NewIndexFamily()
		A.Flex.InsertSlice([]FieldName{"a", "b"})
		A.setLast("c")
		B.Flex.InsertSlice([]FieldName{"a", "b"})
		B.setLast("c")

		output, found := refineIdxFamilies(A, B)
		asserts.True(found, "There is a solution to merge them (identical)")
		asserts.True(output.Equal(B), "Expect to keep the second one")
	})

	t.Run("6", func(t *testing.T) {
		A := NewIndexFamily()
		B := NewIndexFamily()
		A.Flex.InsertSlice([]FieldName{"a", "b"})
		A.setLast("c")
		B.Flex.InsertSlice([]FieldName{"a", "b", "c"})

		output, found := refineIdxFamilies(A, B)
		asserts.True(found, "There is a solution to merge them (identical)")
		asserts.True(output.Equal(A), "Expect to keep the first one")
	})

	t.Run("7", func(t *testing.T) {
		A := NewIndexFamily()
		B := NewIndexFamily()
		A.Flex.InsertSlice([]FieldName{"a", "b"})
		A.setLast("c")
		B.Fixed = []FieldName{"a", "b", "c"}

		output, found := refineIdxFamilies(A, B)
		asserts.True(found, "There is a solution to merge them (identical)")
		asserts.True(output.Equal(B), "Expect to keep the second one")
	})

	t.Run("8", func(t *testing.T) {
		A := NewIndexFamily()
		B := NewIndexFamily()
		A.Flex.InsertSlice([]FieldName{"a", "b"})
		A.setLast("c")
		B.Fixed = []FieldName{"a", "c", "b"}

		_, found := refineIdxFamilies(A, B)
		asserts.False(found, "There no solution to merge them")
	})

	t.Run("9", func(t *testing.T) {
		A := NewIndexFamily()
		B := NewIndexFamily()
		A.Flex.InsertSlice([]FieldName{"b", "c"})
		A.setLast("a")
		B.Fixed = []FieldName{"a"}
		B.Flex.InsertSlice([]FieldName{"b", "c"})

		_, found := refineIdxFamilies(A, B)
		asserts.False(found, "There no solution to merge them")
	})

	t.Run("10", func(t *testing.T) {
		A := NewIndexFamily()
		B := NewIndexFamily()
		A.Flex.InsertSlice([]FieldName{"a", "b"})
		B.Fixed = []FieldName{"a", "b", "c"}

		output, found := refineIdxFamilies(A, B)
		asserts.True(found, "There is a solution to merge them")
		asserts.True(output.Equal(B), "Expect to keep the second one")
	})

	t.Run("11", func(t *testing.T) {
		A := NewIndexFamily()
		B := NewIndexFamily()
		A.Flex.InsertSlice([]FieldName{"a", "d"})
		B.Fixed = []FieldName{"a", "b", "c"}

		_, found := refineIdxFamilies(A, B)
		asserts.False(found, "There is no solution to merge them")
	})

	t.Run("12", func(t *testing.T) {
		A := NewIndexFamily()
		B := NewIndexFamily()
		A.Flex.InsertSlice([]FieldName{"a"})
		A.setLast("d")
		B.Fixed = []FieldName{"a", "b", "c"}

		_, found := refineIdxFamilies(A, B)
		asserts.False(found, "There is no solution to merge them")
	})

	t.Run("13", func(t *testing.T) {
		A := NewIndexFamily()
		B := NewIndexFamily()
		A.Flex.InsertSlice([]FieldName{"a", "b", "c"})
		A.setLast("d")
		B.Fixed = []FieldName{"a", "b"}
		B.Flex.InsertSlice([]FieldName{"c", "e", "f", "g"})

		_, found := refineIdxFamilies(A, B)
		asserts.False(found, "There is no solution to merge them")
	})

	t.Run("14", func(t *testing.T) {
		A := NewIndexFamily()
		B := NewIndexFamily()
		A.Flex.InsertSlice([]FieldName{"a", "b", "c"})
		A.setLast("d")
		B.Fixed = []FieldName{"a", "b"}
		B.Flex.InsertSlice([]FieldName{"c", "d", "e", "f", "g"})

		output, found := refineIdxFamilies(A, B)
		asserts.True(found, "There is a solution to merge them")
		expected := NewIndexFamily()
		expected.Fixed = []FieldName{"a", "b", "c", "d"}
		expected.Flex.InsertSlice([]FieldName{"e", "f", "g"})
		asserts.True(output.Equal(expected), "Expected value")
	})

	t.Run("15", func(t *testing.T) {
		A := NewIndexFamily()
		B := NewIndexFamily()
		A.Flex.InsertSlice([]FieldName{"a", "b", "c"})
		A.setLast("d")
		B.Fixed = []FieldName{"a", "b", "c", "d", "e"}
		B.Flex.InsertSlice([]FieldName{"f", "g"})

		output, found := refineIdxFamilies(A, B)
		asserts.True(found, "There is a solution to merge them")
		expected := NewIndexFamily()
		expected.Fixed = []FieldName{"a", "b", "c", "d", "e"}
		expected.Flex.InsertSlice([]FieldName{"f", "g"})
		asserts.True(output.Equal(expected), "Expected value")
	})

	t.Run("16", func(t *testing.T) {
		A := NewIndexFamily()
		B := NewIndexFamily()
		A.Flex.InsertSlice([]FieldName{"a", "b", "c"})
		A.setLast("d")
		A.Included.InsertSlice([]FieldName{"x", "y", "e"})
		B.Fixed = []FieldName{"a", "b", "c", "d", "e"}
		B.Flex.InsertSlice([]FieldName{"f", "g"})
		B.Included.InsertSlice([]FieldName{"x", "z"})

		output, found := refineIdxFamilies(A, B)
		asserts.True(found, "There is a solution to merge them")
		expected := NewIndexFamily()
		expected.Fixed = []FieldName{"a", "b", "c", "d", "e"}
		expected.Flex.InsertSlice([]FieldName{"f", "g"})
		expected.Included.InsertSlice([]FieldName{"x", "y", "z"})
		asserts.True(output.Equal(expected), "Expected value")
	})

	t.Run("17", func(t *testing.T) {
		A := NewIndexFamily()
		B := NewIndexFamily()
		A.Flex.InsertSlice([]FieldName{"a", "b"})
		A.setLast("c")
		B.Flex.InsertSlice([]FieldName{"a", "b", "d"})
		B.setLast("c")

		output, found := refineIdxFamilies(B, A)
		fmt.Println(output)
		asserts.False(found, "There is no solution to merge them")
	})

	t.Run("18", func(t *testing.T) {
		A := NewIndexFamily()
		B := NewIndexFamily()
		A.Flex.InsertSlice([]FieldName{"a"})
		A.Fixed = []FieldName{"e"}
		A.setLast("c")
		B.Flex.InsertSlice([]FieldName{"a", "b", "d"})
		B.setLast("c")

		output, found := refineIdxFamilies(B, A)
		fmt.Println(output)
		asserts.False(found, "There is no solution to merge them")
	})
}
