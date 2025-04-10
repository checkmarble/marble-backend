package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCovers(t *testing.T) {
	idx := ConcreteIndex{
		TableName: "table",
		Indexed:   []string{"a", "b", "c", "d", "e"},
		Included:  []string{"f", "g"},
	}

	t.Run("With fixed & flex & last - true", func(t *testing.T) {
		asserts := assert.New(t)
		family := NewIndexFamily()
		family.TableName = "table"
		family.Fixed = []string{"a", "b"}
		family.Flex.InsertSlice([]string{"c", "d"})
		family.SetLast("e")
		family.Included.InsertSlice([]string{"f"})

		asserts.True(idx.Covers(family), "The index is an instance of the family")
	})

	t.Run("With fixed & flex & last - false", func(t *testing.T) {
		asserts := assert.New(t)
		family := NewIndexFamily()
		family.TableName = "table"
		family.Fixed = []string{"a", "b"}
		family.Flex.InsertSlice([]string{"c"})
		family.SetLast("X")
		family.Included.InsertSlice([]string{"f"})

		asserts.False(idx.Covers(family), "The index not an instance of the family")
	})

	t.Run("With fixed & last - true", func(t *testing.T) {
		asserts := assert.New(t)
		family := NewIndexFamily()
		family.TableName = "table"
		family.Fixed = []string{"a", "b", "c"}
		family.SetLast("d")
		family.Included.InsertSlice([]string{"f"})

		asserts.True(idx.Covers(family), "The index is an instance of the family")
	})

	t.Run("With fixed & last - false", func(t *testing.T) {
		asserts := assert.New(t)
		family := NewIndexFamily()
		family.TableName = "table"
		family.Fixed = []string{"a", "b", "c"}
		family.SetLast("e")
		family.Included.InsertSlice([]string{"f"})

		asserts.False(idx.Covers(family), "The index is not an instance of the family")
	})

	t.Run("With fixed - true", func(t *testing.T) {
		asserts := assert.New(t)
		family := NewIndexFamily()
		family.TableName = "table"
		family.Fixed = []string{"a", "b", "c", "d"}
		family.Included.InsertSlice([]string{"f"})

		asserts.True(idx.Covers(family), "The index is an instance of the family")
	})

	t.Run("With fixed - false", func(t *testing.T) {
		asserts := assert.New(t)
		family := NewIndexFamily()
		family.TableName = "table"
		family.Fixed = []string{"a", "b", "d"}
		family.Included.InsertSlice([]string{"f"})

		asserts.False(idx.Covers(family), "The index is not an instance of the family")
	})

	t.Run("With fixed & flex - true 1", func(t *testing.T) {
		asserts := assert.New(t)
		family := NewIndexFamily()
		family.TableName = "table"
		family.Fixed = []string{"a", "b"}
		family.Flex.InsertSlice([]string{"c", "d"})
		family.Included.InsertSlice([]string{"f"})

		asserts.True(idx.Covers(family), "The index is an instance of the family")
	})

	t.Run("With fixed & flex - true 2", func(t *testing.T) {
		asserts := assert.New(t)
		family := NewIndexFamily()
		family.TableName = "table"
		family.Fixed = []string{"a", "b"}
		family.Flex.InsertSlice([]string{"c"})
		family.Included.InsertSlice([]string{"f"})

		asserts.True(idx.Covers(family), "The index is an instance of the family")
	})

	t.Run("With fixed & flex - false", func(t *testing.T) {
		asserts := assert.New(t)
		family := NewIndexFamily()
		family.TableName = "table"
		family.Fixed = []string{"a", "b"}
		family.Flex.InsertSlice([]string{"c", "e"})
		family.Included.InsertSlice([]string{"f"})

		asserts.False(idx.Covers(family), "The index is not an instance of the family")
	})

	t.Run("With flex & last - true", func(t *testing.T) {
		asserts := assert.New(t)
		family := NewIndexFamily()
		family.TableName = "table"
		family.Flex.InsertSlice([]string{"a", "b"})
		family.SetLast("c")
		family.Included.InsertSlice([]string{"f"})

		asserts.True(idx.Covers(family), "The index is an instance of the family")
	})

	t.Run("With flex & last - false", func(t *testing.T) {
		asserts := assert.New(t)
		family := NewIndexFamily()
		family.TableName = "table"
		family.Flex.InsertSlice([]string{"a", "b"})
		family.SetLast("d")
		family.Included.InsertSlice([]string{"f"})

		asserts.False(idx.Covers(family), "The index is not an instance of the family")
	})

	t.Run("With only flex - true", func(t *testing.T) {
		asserts := assert.New(t)
		family := NewIndexFamily()
		family.TableName = "table"
		family.Flex.InsertSlice([]string{"a", "b", "c"})
		family.Included.InsertSlice([]string{"f"})

		asserts.True(idx.Covers(family), "The index is an instance of the family")
	})

	t.Run("With only flex - false", func(t *testing.T) {
		asserts := assert.New(t)
		family := NewIndexFamily()
		family.TableName = "table"
		family.Flex.InsertSlice([]string{"a", "b", "d"})
		family.Included.InsertSlice([]string{"f"})

		asserts.False(idx.Covers(family), "The index is not an instance of the family")
	})

	t.Run("Check on included columns - false", func(t *testing.T) {
		asserts := assert.New(t)
		family := NewIndexFamily()
		family.TableName = "table"
		family.Flex.InsertSlice([]string{"a", "b", "c"})
		family.Included.InsertSlice([]string{"f", "h"})

		asserts.False(idx.Covers(family), "The index is not an instance of the family")
	})

	t.Run("Column names are not case sensitive", func(t *testing.T) {
		asserts := assert.New(t)
		family := NewIndexFamily()
		family.TableName = "table"
		family.Flex.InsertSlice([]string{"A", "B", "c"})
		family.Included.InsertSlice([]string{"f", "g"})
		family.Last = "D"

		asserts.True(idx.Covers(family), "The index is an instance of the family")
	})

	t.Run("'Included' columns from index family can also just be in the index - 1", func(t *testing.T) {
		asserts := assert.New(t)
		family := NewIndexFamily()
		family.TableName = "table"
		family.Flex.InsertSlice([]string{"a", "b", "c"})
		family.Included.InsertSlice([]string{"d"})

		asserts.True(idx.Covers(family), "The index is an instance of the family")
	})

	t.Run("'Included' columns from index family can also just be in the index - 2", func(t *testing.T) {
		asserts := assert.New(t)
		family := NewIndexFamily()
		family.TableName = "table"
		family.Fixed = []string{"a", "b", "c"}
		family.Included.InsertSlice([]string{"d"})

		asserts.True(idx.Covers(family), "The index is an instance of the family")
	})
}
