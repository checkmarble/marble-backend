package indexes

import (
	"fmt"
	"testing"

	"github.com/checkmarble/marble-backend/models"
	"github.com/hashicorp/go-set/v2"
	"github.com/stretchr/testify/assert"
)

func TestExtractMinimalSetOfIdxFamilies(t *testing.T) {
	t.Run("Just one input family", func(t *testing.T) {
		asserts := assert.New(t)
		idxFamilies := set.HashSetFrom([]models.IndexFamily{
			{
				Fixed:    []string{"a", "b"},
				Flex:     set.From([]string{"c"}),
				Last:     "d",
				Included: set.From([]string{"e", "f"}),
			},
		})

		minimalSet := extractMinimalSetOfIdxFamilies(idxFamilies)
		asserts.Equal(1, minimalSet.Size(), "Keep just the one input family")
		asserts.True(minimalSet.EqualSet(idxFamilies),
			"The input and output sets are the same")
	})

	t.Run("2 non overlapping families", func(t *testing.T) {
		asserts := assert.New(t)
		idxFamilies := set.HashSetFrom([]models.IndexFamily{
			{
				Fixed:    []string{},
				Flex:     set.From([]string{"a"}),
				Last:     "",
				Included: set.New[string](0),
			},
			{
				Fixed:    []string{},
				Flex:     set.From([]string{"b"}),
				Last:     "",
				Included: set.New[string](0),
			},
		})

		minimalSet := extractMinimalSetOfIdxFamilies(idxFamilies)
		asserts.Equal(2, minimalSet.Size(), "Keep the two identical input families")
		asserts.True(minimalSet.EqualSet(idxFamilies),
			"The input and output sets are the same")
	})

	t.Run("2 overlapping families", func(t *testing.T) {
		asserts := assert.New(t)
		expected := set.HashSetFrom([]models.IndexFamily{{
			Fixed:    []string{},
			Flex:     set.From([]string{"a", "b"}),
			Last:     "",
			Included: set.New[string](0),
		}})
		idxFamilies := set.HashSetFrom([]models.IndexFamily{
			{
				Fixed:    []string{},
				Flex:     set.From([]string{"a"}),
				Last:     "",
				Included: set.New[string](0),
			},
		})

		minimalSet := extractMinimalSetOfIdxFamilies(
			idxFamilies.Union(expected).(*set.HashSet[models.IndexFamily, string]))
		asserts.Equal(1, minimalSet.Size(), "Keep just one idx family")
		asserts.True(minimalSet.EqualSet(expected), "Keep the second input idx families")
	})
}

func TestRefineIdxFamiliesShortHasNoFixed(t *testing.T) {
	asserts := assert.New(t)

	t.Run("1: not the same columns indexed", func(t *testing.T) {
		A := models.NewIndexFamily()
		B := models.NewIndexFamily()
		A.Flex.Insert("a")
		B.Flex.Insert("b")

		_, found := refineIdxFamilies(A, B)
		asserts.False(found, "No overlap expected")
	})

	t.Run("2: identical, no \"last\"", func(t *testing.T) {
		A := models.NewIndexFamily()
		B := models.NewIndexFamily()
		A.Flex.Insert("a")
		B.Flex.Insert("a")

		output, found := refineIdxFamilies(A, B)
		asserts.True(found, "Expect identical output")
		asserts.True(output.Equal(A), "Expect identical output")
	})

	t.Run("3: A is like B without order, no \"last\"", func(t *testing.T) {
		A := models.NewIndexFamily()
		B := models.NewIndexFamily()
		A.Flex.Insert("a")
		B.Fixed = []string{"a"}

		output, found := refineIdxFamilies(A, B)
		asserts.True(found, "Expect identical output")
		asserts.True(output.Equal(B), "Expect identical output")
	})

	t.Run("Simple case 2: simple overlap", func(t *testing.T) {
		fam1 := models.NewIndexFamily()
		fam2 := models.NewIndexFamily()
		fam1.Flex.Insert("a")
		fam2.Flex.InsertSlice([]string{"a", "b"})

		output, found := refineIdxFamilies(fam1, fam2)
		asserts.True(found, "Expect identical output")
		asserts.True(output.Equal(fam2), "Expect to keep the second one")
	})

	t.Run("Simple case 2: simple overlap 3", func(t *testing.T) {
		fam1 := models.NewIndexFamily()
		fam2 := models.NewIndexFamily()
		fam1.Flex.InsertSlice([]string{"a", "c"})
		fam2.Flex.InsertSlice([]string{"a", "b", "c"})

		output, found := refineIdxFamilies(fam1, fam2)
		asserts.True(found, "Expect identical output")
		asserts.True(output.Equal(fam2), "Expect to keep the second one")
	})

	t.Run("Disjoint: no way to combine flex that are disjoint", func(t *testing.T) {
		fam1 := models.NewIndexFamily()
		fam2 := models.NewIndexFamily()
		fam1.Flex.InsertSlice([]string{"a", "c"})
		fam2.Flex.InsertSlice([]string{"a", "b"})

		_, found := refineIdxFamilies(fam1, fam2)
		asserts.False(found, "No solution to merge them")
	})

	t.Run("With fixed: 1", func(t *testing.T) {
		fam1 := models.NewIndexFamily()
		fam2 := models.NewIndexFamily()
		fam1.Fixed = []string{"a", "b"}
		fam1.Flex.InsertSlice([]string{"c", "d"})
		fam1.SetLast("e")
		fam2.Fixed = []string{"a", "b"}

		output, found := refineIdxFamilies(fam1, fam2)
		asserts.True(found, "There is a trivial solution to merge them")
		asserts.True(output.Equal(fam1), "Expect to keep the first one")
	})

	t.Run("With fixed: 2", func(t *testing.T) {
		fam1 := models.NewIndexFamily()
		fam2 := models.NewIndexFamily()
		fam1.Fixed = []string{"a", "b"}
		fam1.Flex.InsertSlice([]string{"c", "d"})
		fam1.SetLast("e")
		fam2.Fixed = []string{"a", "b"}
		fam2.SetLast("c")

		output, found := refineIdxFamilies(fam1, fam2)
		asserts.True(found, "There is a solution to merge them")
		expected := models.NewIndexFamily()
		expected.Fixed = []string{"a", "b", "c"}
		expected.Flex.InsertSlice([]string{"d"})
		expected.SetLast("e")
		asserts.True(output.Equal(expected), "Expect to keep the first one")
	})

	t.Run("With fixed incompatible", func(t *testing.T) {
		fam1 := models.NewIndexFamily()
		fam2 := models.NewIndexFamily()
		fam1.Fixed = []string{"a", "b"}
		fam1.Flex.InsertSlice([]string{"c", "d"})
		fam1.SetLast("e")
		fam2.Fixed = []string{"a", "c"}
		fam2.SetLast("c")

		_, found := refineIdxFamilies(fam1, fam2)
		asserts.False(found, "No solution to merge them")
	})

	t.Run("With fixed incompatible", func(t *testing.T) {
		fam1 := models.NewIndexFamily()
		fam2 := models.NewIndexFamily()
		fam1.Fixed = []string{"a", "b"}
		fam1.Flex.InsertSlice([]string{"c", "d"})
		fam1.SetLast("e")
		fam2.Fixed = []string{"a", "c"}
		fam2.SetLast("c")

		_, found := refineIdxFamilies(fam1, fam2)
		asserts.False(found, "No solution to merge them")
	})

	t.Run("1", func(t *testing.T) {
		A := models.NewIndexFamily()
		B := models.NewIndexFamily()
		A.Fixed = []string{"a", "b"}
		A.Flex.InsertSlice([]string{"c", "d"})
		A.SetLast("e")
		B.Flex.InsertSlice([]string{"a", "c", "f"})

		_, found := refineIdxFamilies(A, B)
		asserts.False(found, "No solution to merge them")
	})

	t.Run("2", func(t *testing.T) {
		A := models.NewIndexFamily()
		B := models.NewIndexFamily()
		A.Flex.InsertSlice([]string{"a", "b", "c", "d"})
		A.SetLast("e")
		B.Flex.InsertSlice([]string{"a", "c", "f"})

		_, found := refineIdxFamilies(A, B)
		asserts.False(found, "No solution to merge them")
	})

	t.Run("3", func(t *testing.T) {
		A := models.NewIndexFamily()
		B := models.NewIndexFamily()
		A.Flex.InsertSlice([]string{"a", "b", "c", "d"})
		A.SetLast("e")
		B.Flex.InsertSlice([]string{"a", "b", "c"})
		B.SetLast("d")

		output, found := refineIdxFamilies(A, B)
		asserts.True(found, "There is a solution to merge them")
		expected := models.NewIndexFamily()
		expected.Fixed = []string{"a", "b", "c", "d"}
		expected.SetLast("e")
		asserts.True(output.Equal(expected), "Expect to keep the first one")
	})

	t.Run("4", func(t *testing.T) {
		A := models.NewIndexFamily()
		B := models.NewIndexFamily()
		A.Flex.InsertSlice([]string{"a", "b"})
		A.SetLast("c")
		B.Flex.InsertSlice([]string{"a", "c"})
		B.SetLast("b")

		_, found := refineIdxFamilies(A, B)
		asserts.False(found, "There is no solution to merge them")
	})

	t.Run("5", func(t *testing.T) {
		A := models.NewIndexFamily()
		B := models.NewIndexFamily()
		A.Flex.InsertSlice([]string{"a", "b"})
		A.SetLast("c")
		B.Flex.InsertSlice([]string{"a", "b"})
		B.SetLast("c")

		output, found := refineIdxFamilies(A, B)
		asserts.True(found, "There is a solution to merge them (identical)")
		asserts.True(output.Equal(B), "Expect to keep the second one")
	})

	t.Run("6", func(t *testing.T) {
		A := models.NewIndexFamily()
		B := models.NewIndexFamily()
		A.Flex.InsertSlice([]string{"a", "b"})
		A.SetLast("c")
		B.Flex.InsertSlice([]string{"a", "b", "c"})

		output, found := refineIdxFamilies(A, B)
		asserts.True(found, "There is a solution to merge them (identical)")
		asserts.True(output.Equal(A), "Expect to keep the first one")
	})

	t.Run("7", func(t *testing.T) {
		A := models.NewIndexFamily()
		B := models.NewIndexFamily()
		A.Flex.InsertSlice([]string{"a", "b"})
		A.SetLast("c")
		B.Fixed = []string{"a", "b", "c"}

		output, found := refineIdxFamilies(A, B)
		asserts.True(found, "There is a solution to merge them (identical)")
		asserts.True(output.Equal(B), "Expect to keep the second one")
	})

	t.Run("8", func(t *testing.T) {
		A := models.NewIndexFamily()
		B := models.NewIndexFamily()
		A.Flex.InsertSlice([]string{"a", "b"})
		A.SetLast("c")
		B.Fixed = []string{"a", "c", "b"}

		_, found := refineIdxFamilies(A, B)
		asserts.False(found, "There no solution to merge them")
	})

	t.Run("9", func(t *testing.T) {
		A := models.NewIndexFamily()
		B := models.NewIndexFamily()
		A.Flex.InsertSlice([]string{"b", "c"})
		A.SetLast("a")
		B.Fixed = []string{"a"}
		B.Flex.InsertSlice([]string{"b", "c"})

		_, found := refineIdxFamilies(A, B)
		asserts.False(found, "There no solution to merge them")
	})

	t.Run("10", func(t *testing.T) {
		A := models.NewIndexFamily()
		B := models.NewIndexFamily()
		A.Flex.InsertSlice([]string{"a", "b"})
		B.Fixed = []string{"a", "b", "c"}

		output, found := refineIdxFamilies(A, B)
		asserts.True(found, "There is a solution to merge them")
		asserts.True(output.Equal(B), "Expect to keep the second one")
	})

	t.Run("11", func(t *testing.T) {
		A := models.NewIndexFamily()
		B := models.NewIndexFamily()
		A.Flex.InsertSlice([]string{"a", "d"})
		B.Fixed = []string{"a", "b", "c"}

		_, found := refineIdxFamilies(A, B)
		asserts.False(found, "There is no solution to merge them")
	})

	t.Run("12", func(t *testing.T) {
		A := models.NewIndexFamily()
		B := models.NewIndexFamily()
		A.Flex.InsertSlice([]string{"a"})
		A.SetLast("d")
		B.Fixed = []string{"a", "b", "c"}

		_, found := refineIdxFamilies(A, B)
		asserts.False(found, "There is no solution to merge them")
	})

	t.Run("13", func(t *testing.T) {
		A := models.NewIndexFamily()
		B := models.NewIndexFamily()
		A.Flex.InsertSlice([]string{"a", "b", "c"})
		A.SetLast("d")
		B.Fixed = []string{"a", "b"}
		B.Flex.InsertSlice([]string{"c", "e", "f", "g"})

		_, found := refineIdxFamilies(A, B)
		asserts.False(found, "There is no solution to merge them")
	})

	t.Run("14", func(t *testing.T) {
		A := models.NewIndexFamily()
		B := models.NewIndexFamily()
		A.Flex.InsertSlice([]string{"a", "b", "c"})
		A.SetLast("d")
		B.Fixed = []string{"a", "b"}
		B.Flex.InsertSlice([]string{"c", "d", "e", "f", "g"})

		output, found := refineIdxFamilies(A, B)
		asserts.True(found, "There is a solution to merge them")
		expected := models.NewIndexFamily()
		expected.Fixed = []string{"a", "b", "c", "d"}
		expected.Flex.InsertSlice([]string{"e", "f", "g"})
		asserts.True(output.Equal(expected), "Expected value")
	})

	t.Run("15", func(t *testing.T) {
		A := models.NewIndexFamily()
		B := models.NewIndexFamily()
		A.Flex.InsertSlice([]string{"a", "b", "c"})
		A.SetLast("d")
		B.Fixed = []string{"a", "b", "c", "d", "e"}
		B.Flex.InsertSlice([]string{"f", "g"})

		output, found := refineIdxFamilies(A, B)
		asserts.True(found, "There is a solution to merge them")
		expected := models.NewIndexFamily()
		expected.Fixed = []string{"a", "b", "c", "d", "e"}
		expected.Flex.InsertSlice([]string{"f", "g"})
		asserts.True(output.Equal(expected), "Expected value")
	})

	t.Run("16", func(t *testing.T) {
		A := models.NewIndexFamily()
		B := models.NewIndexFamily()
		A.Flex.InsertSlice([]string{"a", "b", "c"})
		A.SetLast("d")
		A.Included.InsertSlice([]string{"x", "y", "e"})
		B.Fixed = []string{"a", "b", "c", "d", "e"}
		B.Flex.InsertSlice([]string{"f", "g"})
		B.Included.InsertSlice([]string{"x", "z"})

		output, found := refineIdxFamilies(A, B)
		asserts.True(found, "There is a solution to merge them")
		expected := models.NewIndexFamily()
		expected.Fixed = []string{"a", "b", "c", "d", "e"}
		expected.Flex.InsertSlice([]string{"f", "g"})
		expected.Included.InsertSlice([]string{"x", "y", "z"})
		asserts.True(output.Equal(expected), "Expected value")
	})

	t.Run("17", func(t *testing.T) {
		A := models.NewIndexFamily()
		B := models.NewIndexFamily()
		A.Flex.InsertSlice([]string{"a", "b"})
		A.SetLast("c")
		B.Flex.InsertSlice([]string{"a", "b", "d"})
		B.SetLast("c")

		output, found := refineIdxFamilies(B, A)
		fmt.Println(output)
		asserts.False(found, "There is no solution to merge them")
	})

	t.Run("18", func(t *testing.T) {
		A := models.NewIndexFamily()
		B := models.NewIndexFamily()
		A.Flex.InsertSlice([]string{"a"})
		A.Fixed = []string{"e"}
		A.SetLast("c")
		B.Flex.InsertSlice([]string{"a", "b", "d"})
		B.SetLast("c")

		output, found := refineIdxFamilies(B, A)
		fmt.Println(output)
		asserts.False(found, "There is no solution to merge them")
	})

	t.Run("19", func(t *testing.T) {
		A := models.NewIndexFamily()
		B := models.NewIndexFamily()
		A.Flex.InsertSlice([]string{"a", "b", "c", "d", "e"})
		// A.SetLast("c")
		B.Fixed = []string{"a"}
		B.Flex.InsertSlice([]string{"b", "c"})
		B.SetLast("d")

		output, found := refineIdxFamilies(A, B)
		fmt.Println(output)
		asserts.True(found, "There is a solution to merge them")
	})

	t.Run("20", func(t *testing.T) {
		A := models.NewIndexFamily()
		B := models.NewIndexFamily()
		A.Flex.InsertSlice([]string{"a", "b", "c", "d"})
		A.SetLast("e")
		B.Fixed = []string{"a"}
		B.Flex.InsertSlice([]string{"b", "c"})
		B.SetLast("e")

		output, found := refineIdxFamilies(A, B)
		fmt.Println(output)
		asserts.False(found, "There is a solution to merge them")
	})

	t.Run("21", func(t *testing.T) {
		A := models.NewIndexFamily()
		B := models.NewIndexFamily()
		A.Flex.InsertSlice([]string{"a", "b", "c"})
		A.SetLast("d")
		B.Fixed = []string{"a"}
		B.Flex.InsertSlice([]string{"b", "c", "d"})

		output, found := refineIdxFamilies(A, B)
		fmt.Println(output)
		asserts.True(found, "There is a solution to merge them")
	})

	t.Run("22", func(t *testing.T) {
		A := models.NewIndexFamily()
		B := models.NewIndexFamily()
		A.Flex.InsertSlice([]string{"a", "b", "c"})
		B.Fixed = []string{"a", "b", "d", "c"}

		output, found := refineIdxFamilies(A, B)
		fmt.Println(output)
		asserts.False(found, "There is no solution to merge them")
	})

	t.Run("23", func(t *testing.T) {
		A := models.NewIndexFamily()
		B := models.NewIndexFamily()
		A.Flex.InsertSlice([]string{"a", "b"})
		A.Last = "c"
		B.Fixed = []string{"a", "b", "d", "c"}

		output, found := refineIdxFamilies(A, B)
		fmt.Println(output)
		asserts.False(found, "There is no solution to merge them")
	})

	t.Run("24", func(t *testing.T) {
		A := models.NewIndexFamily()
		B := models.NewIndexFamily()
		A.Flex.InsertSlice([]string{"a", "b", "c"})
		B.Fixed = []string{"a", "g"}
		B.Flex.InsertSlice([]string{"b", "c", "d", "e", "f", "g", "h"})

		output, found := refineIdxFamilies(A, B)
		fmt.Println(output)
		asserts.False(found, "There is no solution to merge them")
	})
}
