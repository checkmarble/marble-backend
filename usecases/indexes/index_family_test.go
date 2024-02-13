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
		idxFamilies := set.HashSetFrom[models.IndexFamily]([]models.IndexFamily{
			{
				Fixed:    []models.FieldName{models.FieldName("a"), models.FieldName("b")},
				Flex:     set.From[models.FieldName]([]models.FieldName{models.FieldName("c")}),
				Last:     models.FieldName("d"),
				Included: set.From[models.FieldName]([]models.FieldName{models.FieldName("e"), models.FieldName("f")}),
			},
		})

		minimalSet := extractMinimalSetOfIdxFamilies(idxFamilies)
		asserts.Equal(1, minimalSet.Size(), "Keep just the one input family")
		asserts.True(minimalSet.EqualSet(idxFamilies), "The input and output sets are the same")
	})

	t.Run("2 non overlapping families", func(t *testing.T) {
		asserts := assert.New(t)
		idxFamilies := set.HashSetFrom[models.IndexFamily]([]models.IndexFamily{
			{
				Fixed:    []models.FieldName{},
				Flex:     set.From[models.FieldName]([]models.FieldName{models.FieldName("a")}),
				Last:     models.FieldName(""),
				Included: set.New[models.FieldName](0),
			},
			{
				Fixed:    []models.FieldName{},
				Flex:     set.From[models.FieldName]([]models.FieldName{models.FieldName("b")}),
				Last:     models.FieldName(""),
				Included: set.New[models.FieldName](0),
			},
		})

		minimalSet := extractMinimalSetOfIdxFamilies(idxFamilies)
		asserts.Equal(2, minimalSet.Size(), "Keep the two identical input families")
		asserts.True(minimalSet.EqualSet(idxFamilies), "The input and output sets are the same")
	})

	t.Run("2 overlapping families", func(t *testing.T) {
		asserts := assert.New(t)
		expected := set.HashSetFrom[models.IndexFamily]([]models.IndexFamily{{
			Fixed:    []models.FieldName{},
			Flex:     set.From[models.FieldName]([]models.FieldName{models.FieldName("a"), models.FieldName("b")}),
			Last:     models.FieldName(""),
			Included: set.New[models.FieldName](0),
		}})
		idxFamilies := set.HashSetFrom[models.IndexFamily]([]models.IndexFamily{
			{
				Fixed:    []models.FieldName{},
				Flex:     set.From[models.FieldName]([]models.FieldName{models.FieldName("a")}),
				Last:     models.FieldName(""),
				Included: set.New[models.FieldName](0),
			},
		})

		minimalSet := extractMinimalSetOfIdxFamilies(idxFamilies.Union(expected).(*set.HashSet[models.IndexFamily, string]))
		asserts.Equal(1, minimalSet.Size(), "Keep just one idx family")
		asserts.True(minimalSet.EqualSet(expected), "Keep the second input idx families")
	})

}

func TestRefineIdxFamiliesShortHasNoFixed(t *testing.T) {
	asserts := assert.New(t)

	t.Run("1: not the same columns indexed", func(t *testing.T) {
		A := models.NewIndexFamily()
		B := models.NewIndexFamily()
		A.Flex.Insert(models.FieldName("a"))
		B.Flex.Insert(models.FieldName("b"))

		_, found := refineIdxFamilies(A, B)
		asserts.False(found, "No overlap expected")
	})

	t.Run("2: identical, no \"last\"", func(t *testing.T) {
		A := models.NewIndexFamily()
		B := models.NewIndexFamily()
		A.Flex.Insert(models.FieldName("a"))
		B.Flex.Insert(models.FieldName("a"))

		output, found := refineIdxFamilies(A, B)
		asserts.True(found, "Expect identical output")
		asserts.True(output.Equal(A), "Expect identical output")
	})

	t.Run("3: A is like B without order, no \"last\"", func(t *testing.T) {
		A := models.NewIndexFamily()
		B := models.NewIndexFamily()
		A.Flex.Insert(models.FieldName("a"))
		B.Fixed = []models.FieldName{"a"}

		output, found := refineIdxFamilies(A, B)
		asserts.True(found, "Expect identical output")
		asserts.True(output.Equal(B), "Expect identical output")
	})

	t.Run("Simple case 2: simple overlap", func(t *testing.T) {
		fam1 := models.NewIndexFamily()
		fam2 := models.NewIndexFamily()
		fam1.Flex.Insert(models.FieldName("a"))
		fam2.Flex.InsertSlice([]models.FieldName{"a", "b"})

		output, found := refineIdxFamilies(fam1, fam2)
		asserts.True(found, "Expect identical output")
		asserts.True(output.Equal(fam2), "Expect to keep the second one")
	})

	t.Run("Simple case 2: simple overlap 3", func(t *testing.T) {
		fam1 := models.NewIndexFamily()
		fam2 := models.NewIndexFamily()
		fam1.Flex.InsertSlice([]models.FieldName{"a", "c"})
		fam2.Flex.InsertSlice([]models.FieldName{"a", "b", "c"})

		output, found := refineIdxFamilies(fam1, fam2)
		asserts.True(found, "Expect identical output")
		asserts.True(output.Equal(fam2), "Expect to keep the second one")
	})

	t.Run("Disjoint: no way to combine flex that are disjoint", func(t *testing.T) {
		fam1 := models.NewIndexFamily()
		fam2 := models.NewIndexFamily()
		fam1.Flex.InsertSlice([]models.FieldName{"a", "c"})
		fam2.Flex.InsertSlice([]models.FieldName{"a", "b"})

		_, found := refineIdxFamilies(fam1, fam2)
		asserts.False(found, "No solution to merge them")
	})

	t.Run("With fixed: 1", func(t *testing.T) {
		fam1 := models.NewIndexFamily()
		fam2 := models.NewIndexFamily()
		fam1.Fixed = []models.FieldName{"a", "b"}
		fam1.Flex.InsertSlice([]models.FieldName{"c", "d"})
		fam1.SetLast("e")
		fam2.Fixed = []models.FieldName{"a", "b"}

		output, found := refineIdxFamilies(fam1, fam2)
		asserts.True(found, "There is a trivial solution to merge them")
		asserts.True(output.Equal(fam1), "Expect to keep the first one")
	})

	t.Run("With fixed: 2", func(t *testing.T) {
		fam1 := models.NewIndexFamily()
		fam2 := models.NewIndexFamily()
		fam1.Fixed = []models.FieldName{"a", "b"}
		fam1.Flex.InsertSlice([]models.FieldName{"c", "d"})
		fam1.SetLast("e")
		fam2.Fixed = []models.FieldName{"a", "b"}
		fam2.SetLast("c")

		output, found := refineIdxFamilies(fam1, fam2)
		asserts.True(found, "There is a solution to merge them")
		expected := models.NewIndexFamily()
		expected.Fixed = []models.FieldName{"a", "b", "c"}
		expected.Flex.InsertSlice([]models.FieldName{"d"})
		expected.SetLast("e")
		asserts.True(output.Equal(expected), "Expect to keep the first one")
	})

	t.Run("With fixed incompatible", func(t *testing.T) {
		fam1 := models.NewIndexFamily()
		fam2 := models.NewIndexFamily()
		fam1.Fixed = []models.FieldName{"a", "b"}
		fam1.Flex.InsertSlice([]models.FieldName{"c", "d"})
		fam1.SetLast("e")
		fam2.Fixed = []models.FieldName{"a", "c"}
		fam2.SetLast("c")

		_, found := refineIdxFamilies(fam1, fam2)
		asserts.False(found, "No solution to merge them")
	})

	t.Run("With fixed incompatible", func(t *testing.T) {
		fam1 := models.NewIndexFamily()
		fam2 := models.NewIndexFamily()
		fam1.Fixed = []models.FieldName{"a", "b"}
		fam1.Flex.InsertSlice([]models.FieldName{"c", "d"})
		fam1.SetLast("e")
		fam2.Fixed = []models.FieldName{"a", "c"}
		fam2.SetLast("c")

		_, found := refineIdxFamilies(fam1, fam2)
		asserts.False(found, "No solution to merge them")
	})

	t.Run("1", func(t *testing.T) {
		A := models.NewIndexFamily()
		B := models.NewIndexFamily()
		A.Fixed = []models.FieldName{"a", "b"}
		A.Flex.InsertSlice([]models.FieldName{"c", "d"})
		A.SetLast("e")
		B.Flex.InsertSlice([]models.FieldName{"a", "c", "f"})

		_, found := refineIdxFamilies(A, B)
		asserts.False(found, "No solution to merge them")
	})

	t.Run("2", func(t *testing.T) {
		A := models.NewIndexFamily()
		B := models.NewIndexFamily()
		A.Flex.InsertSlice([]models.FieldName{"a", "b", "c", "d"})
		A.SetLast("e")
		B.Flex.InsertSlice([]models.FieldName{"a", "c", "f"})

		_, found := refineIdxFamilies(A, B)
		asserts.False(found, "No solution to merge them")
	})

	t.Run("3", func(t *testing.T) {
		A := models.NewIndexFamily()
		B := models.NewIndexFamily()
		A.Flex.InsertSlice([]models.FieldName{"a", "b", "c", "d"})
		A.SetLast("e")
		B.Flex.InsertSlice([]models.FieldName{"a", "b", "c"})
		B.SetLast("d")

		output, found := refineIdxFamilies(A, B)
		asserts.True(found, "There is a solution to merge them")
		expected := models.NewIndexFamily()
		expected.Fixed = []models.FieldName{"a", "b", "c", "d"}
		expected.SetLast("e")
		asserts.True(output.Equal(expected), "Expect to keep the first one")
	})

	t.Run("4", func(t *testing.T) {
		A := models.NewIndexFamily()
		B := models.NewIndexFamily()
		A.Flex.InsertSlice([]models.FieldName{"a", "b"})
		A.SetLast("c")
		B.Flex.InsertSlice([]models.FieldName{"a", "c"})
		B.SetLast("b")

		_, found := refineIdxFamilies(A, B)
		asserts.False(found, "There is no solution to merge them")

	})

	t.Run("5", func(t *testing.T) {
		A := models.NewIndexFamily()
		B := models.NewIndexFamily()
		A.Flex.InsertSlice([]models.FieldName{"a", "b"})
		A.SetLast("c")
		B.Flex.InsertSlice([]models.FieldName{"a", "b"})
		B.SetLast("c")

		output, found := refineIdxFamilies(A, B)
		asserts.True(found, "There is a solution to merge them (identical)")
		asserts.True(output.Equal(B), "Expect to keep the second one")
	})

	t.Run("6", func(t *testing.T) {
		A := models.NewIndexFamily()
		B := models.NewIndexFamily()
		A.Flex.InsertSlice([]models.FieldName{"a", "b"})
		A.SetLast("c")
		B.Flex.InsertSlice([]models.FieldName{"a", "b", "c"})

		output, found := refineIdxFamilies(A, B)
		asserts.True(found, "There is a solution to merge them (identical)")
		asserts.True(output.Equal(A), "Expect to keep the first one")
	})

	t.Run("7", func(t *testing.T) {
		A := models.NewIndexFamily()
		B := models.NewIndexFamily()
		A.Flex.InsertSlice([]models.FieldName{"a", "b"})
		A.SetLast("c")
		B.Fixed = []models.FieldName{"a", "b", "c"}

		output, found := refineIdxFamilies(A, B)
		asserts.True(found, "There is a solution to merge them (identical)")
		asserts.True(output.Equal(B), "Expect to keep the second one")
	})

	t.Run("8", func(t *testing.T) {
		A := models.NewIndexFamily()
		B := models.NewIndexFamily()
		A.Flex.InsertSlice([]models.FieldName{"a", "b"})
		A.SetLast("c")
		B.Fixed = []models.FieldName{"a", "c", "b"}

		_, found := refineIdxFamilies(A, B)
		asserts.False(found, "There no solution to merge them")
	})

	t.Run("9", func(t *testing.T) {
		A := models.NewIndexFamily()
		B := models.NewIndexFamily()
		A.Flex.InsertSlice([]models.FieldName{"b", "c"})
		A.SetLast("a")
		B.Fixed = []models.FieldName{"a"}
		B.Flex.InsertSlice([]models.FieldName{"b", "c"})

		_, found := refineIdxFamilies(A, B)
		asserts.False(found, "There no solution to merge them")
	})

	t.Run("10", func(t *testing.T) {
		A := models.NewIndexFamily()
		B := models.NewIndexFamily()
		A.Flex.InsertSlice([]models.FieldName{"a", "b"})
		B.Fixed = []models.FieldName{"a", "b", "c"}

		output, found := refineIdxFamilies(A, B)
		asserts.True(found, "There is a solution to merge them")
		asserts.True(output.Equal(B), "Expect to keep the second one")
	})

	t.Run("11", func(t *testing.T) {
		A := models.NewIndexFamily()
		B := models.NewIndexFamily()
		A.Flex.InsertSlice([]models.FieldName{"a", "d"})
		B.Fixed = []models.FieldName{"a", "b", "c"}

		_, found := refineIdxFamilies(A, B)
		asserts.False(found, "There is no solution to merge them")
	})

	t.Run("12", func(t *testing.T) {
		A := models.NewIndexFamily()
		B := models.NewIndexFamily()
		A.Flex.InsertSlice([]models.FieldName{"a"})
		A.SetLast("d")
		B.Fixed = []models.FieldName{"a", "b", "c"}

		_, found := refineIdxFamilies(A, B)
		asserts.False(found, "There is no solution to merge them")
	})

	t.Run("13", func(t *testing.T) {
		A := models.NewIndexFamily()
		B := models.NewIndexFamily()
		A.Flex.InsertSlice([]models.FieldName{"a", "b", "c"})
		A.SetLast("d")
		B.Fixed = []models.FieldName{"a", "b"}
		B.Flex.InsertSlice([]models.FieldName{"c", "e", "f", "g"})

		_, found := refineIdxFamilies(A, B)
		asserts.False(found, "There is no solution to merge them")
	})

	t.Run("14", func(t *testing.T) {
		A := models.NewIndexFamily()
		B := models.NewIndexFamily()
		A.Flex.InsertSlice([]models.FieldName{"a", "b", "c"})
		A.SetLast("d")
		B.Fixed = []models.FieldName{"a", "b"}
		B.Flex.InsertSlice([]models.FieldName{"c", "d", "e", "f", "g"})

		output, found := refineIdxFamilies(A, B)
		asserts.True(found, "There is a solution to merge them")
		expected := models.NewIndexFamily()
		expected.Fixed = []models.FieldName{"a", "b", "c", "d"}
		expected.Flex.InsertSlice([]models.FieldName{"e", "f", "g"})
		asserts.True(output.Equal(expected), "Expected value")
	})

	t.Run("15", func(t *testing.T) {
		A := models.NewIndexFamily()
		B := models.NewIndexFamily()
		A.Flex.InsertSlice([]models.FieldName{"a", "b", "c"})
		A.SetLast("d")
		B.Fixed = []models.FieldName{"a", "b", "c", "d", "e"}
		B.Flex.InsertSlice([]models.FieldName{"f", "g"})

		output, found := refineIdxFamilies(A, B)
		asserts.True(found, "There is a solution to merge them")
		expected := models.NewIndexFamily()
		expected.Fixed = []models.FieldName{"a", "b", "c", "d", "e"}
		expected.Flex.InsertSlice([]models.FieldName{"f", "g"})
		asserts.True(output.Equal(expected), "Expected value")
	})

	t.Run("16", func(t *testing.T) {
		A := models.NewIndexFamily()
		B := models.NewIndexFamily()
		A.Flex.InsertSlice([]models.FieldName{"a", "b", "c"})
		A.SetLast("d")
		A.Included.InsertSlice([]models.FieldName{"x", "y", "e"})
		B.Fixed = []models.FieldName{"a", "b", "c", "d", "e"}
		B.Flex.InsertSlice([]models.FieldName{"f", "g"})
		B.Included.InsertSlice([]models.FieldName{"x", "z"})

		output, found := refineIdxFamilies(A, B)
		asserts.True(found, "There is a solution to merge them")
		expected := models.NewIndexFamily()
		expected.Fixed = []models.FieldName{"a", "b", "c", "d", "e"}
		expected.Flex.InsertSlice([]models.FieldName{"f", "g"})
		expected.Included.InsertSlice([]models.FieldName{"x", "y", "z"})
		asserts.True(output.Equal(expected), "Expected value")
	})

	t.Run("17", func(t *testing.T) {
		A := models.NewIndexFamily()
		B := models.NewIndexFamily()
		A.Flex.InsertSlice([]models.FieldName{"a", "b"})
		A.SetLast("c")
		B.Flex.InsertSlice([]models.FieldName{"a", "b", "d"})
		B.SetLast("c")

		output, found := refineIdxFamilies(B, A)
		fmt.Println(output)
		asserts.False(found, "There is no solution to merge them")
	})

	t.Run("18", func(t *testing.T) {
		A := models.NewIndexFamily()
		B := models.NewIndexFamily()
		A.Flex.InsertSlice([]models.FieldName{"a"})
		A.Fixed = []models.FieldName{"e"}
		A.SetLast("c")
		B.Flex.InsertSlice([]models.FieldName{"a", "b", "d"})
		B.SetLast("c")

		output, found := refineIdxFamilies(B, A)
		fmt.Println(output)
		asserts.False(found, "There is no solution to merge them")
	})

	t.Run("19", func(t *testing.T) {
		A := models.NewIndexFamily()
		B := models.NewIndexFamily()
		A.Flex.InsertSlice([]models.FieldName{"a", "b", "c", "d", "e"})
		// A.SetLast("c")
		B.Fixed = []models.FieldName{"a"}
		B.Flex.InsertSlice([]models.FieldName{"b", "c"})
		B.SetLast("d")

		output, found := refineIdxFamilies(A, B)
		fmt.Println(output)
		asserts.True(found, "There is a solution to merge them")
	})

	t.Run("20", func(t *testing.T) {
		A := models.NewIndexFamily()
		B := models.NewIndexFamily()
		A.Flex.InsertSlice([]models.FieldName{"a", "b", "c", "d"})
		A.SetLast("e")
		B.Fixed = []models.FieldName{"a"}
		B.Flex.InsertSlice([]models.FieldName{"b", "c"})
		B.SetLast("e")

		output, found := refineIdxFamilies(A, B)
		fmt.Println(output)
		asserts.False(found, "There is a solution to merge them")
	})

	t.Run("21", func(t *testing.T) {
		A := models.NewIndexFamily()
		B := models.NewIndexFamily()
		A.Flex.InsertSlice([]models.FieldName{"a", "b", "c"})
		A.SetLast("d")
		B.Fixed = []models.FieldName{"a"}
		B.Flex.InsertSlice([]models.FieldName{"b", "c", "d"})

		output, found := refineIdxFamilies(A, B)
		fmt.Println(output)
		asserts.True(found, "There is a solution to merge them")
	})

	t.Run("22", func(t *testing.T) {
		A := models.NewIndexFamily()
		B := models.NewIndexFamily()
		A.Flex.InsertSlice([]models.FieldName{"a", "b", "c"})
		B.Fixed = []models.FieldName{"a", "b", "d", "c"}

		output, found := refineIdxFamilies(A, B)
		fmt.Println(output)
		asserts.False(found, "There is no solution to merge them")
	})

	t.Run("23", func(t *testing.T) {
		A := models.NewIndexFamily()
		B := models.NewIndexFamily()
		A.Flex.InsertSlice([]models.FieldName{"a", "b"})
		A.Last = "c"
		B.Fixed = []models.FieldName{"a", "b", "d", "c"}

		output, found := refineIdxFamilies(A, B)
		fmt.Println(output)
		asserts.False(found, "There is no solution to merge them")
	})

	t.Run("24", func(t *testing.T) {
		A := models.NewIndexFamily()
		B := models.NewIndexFamily()
		A.Flex.InsertSlice([]models.FieldName{"a", "b", "c"})
		B.Fixed = []models.FieldName{"a", "g"}
		B.Flex.InsertSlice([]models.FieldName{"b", "c", "d", "e", "f", "g", "h"})

		output, found := refineIdxFamilies(A, B)
		fmt.Println(output)
		asserts.False(found, "There is no solution to merge them")
	})
}
