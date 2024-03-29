package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCovers(t *testing.T) {
	idx := ConcreteIndex{
		TableName: "table",
		Indexed:   []FieldName{"a", "b", "c", "d", "e"},
		Included:  []FieldName{"f", "g"},
	}

	t.Run("With fixed & flex & last - true", func(t *testing.T) {
		asserts := assert.New(t)
		family := NewIndexFamily()
		family.TableName = "table"
		family.Fixed = []FieldName{"a", "b"}
		family.Flex.InsertSlice([]FieldName{"c", "d"})
		family.SetLast("e")
		family.Included.InsertSlice([]FieldName{"f"})

		asserts.True(idx.Covers(family), "The index is an instance of the family")
	})

	t.Run("With fixed & flex & last - false", func(t *testing.T) {
		asserts := assert.New(t)
		family := NewIndexFamily()
		family.TableName = "table"
		family.Fixed = []FieldName{"a", "b"}
		family.Flex.InsertSlice([]FieldName{"c"})
		family.SetLast("X")
		family.Included.InsertSlice([]FieldName{"f"})

		asserts.False(idx.Covers(family), "The index not an instance of the family")
	})

	t.Run("With fixed & last - true", func(t *testing.T) {
		asserts := assert.New(t)
		family := NewIndexFamily()
		family.TableName = "table"
		family.Fixed = []FieldName{"a", "b", "c"}
		family.SetLast("d")
		family.Included.InsertSlice([]FieldName{"f"})

		asserts.True(idx.Covers(family), "The index is an instance of the family")
	})

	t.Run("With fixed & last - false", func(t *testing.T) {
		asserts := assert.New(t)
		family := NewIndexFamily()
		family.TableName = "table"
		family.Fixed = []FieldName{"a", "b", "c"}
		family.SetLast("e")
		family.Included.InsertSlice([]FieldName{"f"})

		asserts.False(idx.Covers(family), "The index is not an instance of the family")
	})

	t.Run("With fixed - true", func(t *testing.T) {
		asserts := assert.New(t)
		family := NewIndexFamily()
		family.TableName = "table"
		family.Fixed = []FieldName{"a", "b", "c", "d"}
		family.Included.InsertSlice([]FieldName{"f"})

		asserts.True(idx.Covers(family), "The index is an instance of the family")
	})

	t.Run("With fixed - false", func(t *testing.T) {
		asserts := assert.New(t)
		family := NewIndexFamily()
		family.TableName = "table"
		family.Fixed = []FieldName{"a", "b", "d"}
		family.Included.InsertSlice([]FieldName{"f"})

		asserts.False(idx.Covers(family), "The index is not an instance of the family")
	})

	t.Run("With fixed & flex - true 1", func(t *testing.T) {
		asserts := assert.New(t)
		family := NewIndexFamily()
		family.TableName = "table"
		family.Fixed = []FieldName{"a", "b"}
		family.Flex.InsertSlice([]FieldName{"c", "d"})
		family.Included.InsertSlice([]FieldName{"f"})

		asserts.True(idx.Covers(family), "The index is an instance of the family")
	})

	t.Run("With fixed & flex - true 2", func(t *testing.T) {
		asserts := assert.New(t)
		family := NewIndexFamily()
		family.TableName = "table"
		family.Fixed = []FieldName{"a", "b"}
		family.Flex.InsertSlice([]FieldName{"c"})
		family.Included.InsertSlice([]FieldName{"f"})

		asserts.True(idx.Covers(family), "The index is an instance of the family")
	})

	t.Run("With fixed & flex - false", func(t *testing.T) {
		asserts := assert.New(t)
		family := NewIndexFamily()
		family.TableName = "table"
		family.Fixed = []FieldName{"a", "b"}
		family.Flex.InsertSlice([]FieldName{"c", "e"})
		family.Included.InsertSlice([]FieldName{"f"})

		asserts.False(idx.Covers(family), "The index is not an instance of the family")
	})

	t.Run("With flex & last - true", func(t *testing.T) {
		asserts := assert.New(t)
		family := NewIndexFamily()
		family.TableName = "table"
		family.Flex.InsertSlice([]FieldName{"a", "b"})
		family.SetLast("c")
		family.Included.InsertSlice([]FieldName{"f"})

		asserts.True(idx.Covers(family), "The index is an instance of the family")
	})

	t.Run("With flex & last - false", func(t *testing.T) {
		asserts := assert.New(t)
		family := NewIndexFamily()
		family.TableName = "table"
		family.Flex.InsertSlice([]FieldName{"a", "b"})
		family.SetLast("d")
		family.Included.InsertSlice([]FieldName{"f"})

		asserts.False(idx.Covers(family), "The index is not an instance of the family")
	})

	t.Run("With only flex - true", func(t *testing.T) {
		asserts := assert.New(t)
		family := NewIndexFamily()
		family.TableName = "table"
		family.Flex.InsertSlice([]FieldName{"a", "b", "c"})
		family.Included.InsertSlice([]FieldName{"f"})

		asserts.True(idx.Covers(family), "The index is an instance of the family")
	})

	t.Run("With only flex - false", func(t *testing.T) {
		asserts := assert.New(t)
		family := NewIndexFamily()
		family.TableName = "table"
		family.Flex.InsertSlice([]FieldName{"a", "b", "d"})
		family.Included.InsertSlice([]FieldName{"f"})

		asserts.False(idx.Covers(family), "The index is not an instance of the family")
	})

	t.Run("Check on included columns - false", func(t *testing.T) {
		asserts := assert.New(t)
		family := NewIndexFamily()
		family.TableName = "table"
		family.Flex.InsertSlice([]FieldName{"a", "b", "c"})
		family.Included.InsertSlice([]FieldName{"f", "h"})

		asserts.False(idx.Covers(family), "The index is not an instance of the family")
	})

	t.Run("Column names are not case sensitive", func(t *testing.T) {
		asserts := assert.New(t)
		family := NewIndexFamily()
		family.TableName = "table"
		family.Flex.InsertSlice([]FieldName{"A", "B", "c"})
		family.Included.InsertSlice([]FieldName{"f", "g"})
		family.Last = "D"

		asserts.True(idx.Covers(family), "The index is an instance of the family")
	})
}
