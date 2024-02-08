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
		family.setLast("e")
		family.Others.InsertSlice([]FieldName{"f"})

		asserts.True(idx.covers(family), "The index is an instance of the family")
	})

	t.Run("With fixed & flex & last - false", func(t *testing.T) {
		asserts := assert.New(t)
		family := NewIndexFamily()
		family.TableName = "table"
		family.Fixed = []FieldName{"a", "b"}
		family.Flex.InsertSlice([]FieldName{"c"})
		family.setLast("X")
		family.Others.InsertSlice([]FieldName{"f"})

		asserts.False(idx.covers(family), "The index not an instance of the family")
	})

	t.Run("With fixed & last - true", func(t *testing.T) {
		asserts := assert.New(t)
		family := NewIndexFamily()
		family.TableName = "table"
		family.Fixed = []FieldName{"a", "b", "c"}
		family.setLast("d")
		family.Others.InsertSlice([]FieldName{"f"})

		asserts.True(idx.covers(family), "The index is an instance of the family")
	})

	t.Run("With fixed & last - false", func(t *testing.T) {
		asserts := assert.New(t)
		family := NewIndexFamily()
		family.TableName = "table"
		family.Fixed = []FieldName{"a", "b", "c"}
		family.setLast("e")
		family.Others.InsertSlice([]FieldName{"f"})

		asserts.False(idx.covers(family), "The index is not an instance of the family")
	})

	t.Run("With fixed - true", func(t *testing.T) {
		asserts := assert.New(t)
		family := NewIndexFamily()
		family.TableName = "table"
		family.Fixed = []FieldName{"a", "b", "c", "d"}
		family.Others.InsertSlice([]FieldName{"f"})

		asserts.True(idx.covers(family), "The index is an instance of the family")
	})

	t.Run("With fixed - false", func(t *testing.T) {
		asserts := assert.New(t)
		family := NewIndexFamily()
		family.TableName = "table"
		family.Fixed = []FieldName{"a", "b", "d"}
		family.Others.InsertSlice([]FieldName{"f"})

		asserts.False(idx.covers(family), "The index is not an instance of the family")
	})

	t.Run("With fixed & flex - true 1", func(t *testing.T) {
		asserts := assert.New(t)
		family := NewIndexFamily()
		family.TableName = "table"
		family.Fixed = []FieldName{"a", "b"}
		family.Flex.InsertSlice([]FieldName{"c", "d"})
		family.Others.InsertSlice([]FieldName{"f"})

		asserts.True(idx.covers(family), "The index is an instance of the family")
	})

	t.Run("With fixed & flex - true 2", func(t *testing.T) {
		asserts := assert.New(t)
		family := NewIndexFamily()
		family.TableName = "table"
		family.Fixed = []FieldName{"a", "b"}
		family.Flex.InsertSlice([]FieldName{"c"})
		family.Others.InsertSlice([]FieldName{"f"})

		asserts.True(idx.covers(family), "The index is an instance of the family")
	})

	t.Run("With fixed & flex - false", func(t *testing.T) {
		asserts := assert.New(t)
		family := NewIndexFamily()
		family.TableName = "table"
		family.Fixed = []FieldName{"a", "b"}
		family.Flex.InsertSlice([]FieldName{"c", "e"})
		family.Others.InsertSlice([]FieldName{"f"})

		asserts.False(idx.covers(family), "The index is not an instance of the family")
	})

	t.Run("With flex & last - true", func(t *testing.T) {
		asserts := assert.New(t)
		family := NewIndexFamily()
		family.TableName = "table"
		family.Flex.InsertSlice([]FieldName{"a", "b"})
		family.setLast("c")
		family.Others.InsertSlice([]FieldName{"f"})

		asserts.True(idx.covers(family), "The index is an instance of the family")
	})

	t.Run("With flex & last - false", func(t *testing.T) {
		asserts := assert.New(t)
		family := NewIndexFamily()
		family.TableName = "table"
		family.Flex.InsertSlice([]FieldName{"a", "b"})
		family.setLast("d")
		family.Others.InsertSlice([]FieldName{"f"})

		asserts.False(idx.covers(family), "The index is not an instance of the family")
	})

	t.Run("With only flex - true", func(t *testing.T) {
		asserts := assert.New(t)
		family := NewIndexFamily()
		family.TableName = "table"
		family.Flex.InsertSlice([]FieldName{"a", "b", "c"})
		family.Others.InsertSlice([]FieldName{"f"})

		asserts.True(idx.covers(family), "The index is an instance of the family")
	})

	t.Run("With only flex - false", func(t *testing.T) {
		asserts := assert.New(t)
		family := NewIndexFamily()
		family.TableName = "table"
		family.Flex.InsertSlice([]FieldName{"a", "b", "d"})
		family.Others.InsertSlice([]FieldName{"f"})

		asserts.False(idx.covers(family), "The index is not an instance of the family")
	})

	t.Run("Check on included columns - false", func(t *testing.T) {
		asserts := assert.New(t)
		family := NewIndexFamily()
		family.TableName = "table"
		family.Flex.InsertSlice([]FieldName{"a", "b", "c"})
		family.Others.InsertSlice([]FieldName{"f", "h"})

		asserts.False(idx.covers(family), "The index is not an instance of the family")
	})

	t.Run("Column names are not case sensitive", func(t *testing.T) {
		asserts := assert.New(t)
		family := NewIndexFamily()
		family.TableName = "table"
		family.Flex.InsertSlice([]FieldName{"A", "B", "c"})
		family.Others.InsertSlice([]FieldName{"f", "g"})
		family.Last = "D"

		asserts.True(idx.covers(family), "The index is an instance of the family")
	})
}
