package models

import (
	"fmt"
	"testing"

	"github.com/hashicorp/go-set/v2"
	"github.com/stretchr/testify/assert"
)

func TestAggregateQueryToIndexFamily(t *testing.T) {

	t.Run("Case with 2 inequality conditions", func(t *testing.T) {
		asserts := assert.New(t)

		qFamily := AggregateQueryFamily{
			TableName:               "",
			EqConditions:            set.From[FieldName]([]FieldName{FieldName("a"), FieldName("b")}),
			IneqConditions:          set.From[FieldName]([]FieldName{FieldName("c"), FieldName("d")}),
			SelectOrOtherConditions: set.From[FieldName]([]FieldName{FieldName("e"), FieldName("f")}),
		}

		idxFamily := qFamily.ToIndexFamilies()
		asserts.Equal(2, idxFamily.Size(), "Two possible inequality conditions, so 2 families are rendered")
		expected := set.HashSetFrom[IndexFamily]([]IndexFamily{
			{
				Fixed:    []FieldName{},
				Flex:     set.From([]FieldName{FieldName("a"), FieldName("b")}),
				Last:     FieldName("c"),
				Included: set.From[FieldName]([]FieldName{FieldName("d"), FieldName("e"), FieldName("f")}),
			},
			{
				Fixed:    []FieldName{},
				Flex:     set.From([]FieldName{FieldName("a"), FieldName("b")}),
				Last:     FieldName("d"),
				Included: set.From[FieldName]([]FieldName{FieldName("c"), FieldName("e"), FieldName("f")}),
			},
		})
		asserts.True(expected.Equal(idxFamily), "The index families in the result are the expected ones")
	})

	t.Run("Case with 1 inequality conditions", func(t *testing.T) {
		asserts := assert.New(t)

		qFamily := AggregateQueryFamily{
			TableName:               "",
			EqConditions:            set.From[FieldName]([]FieldName{FieldName("a"), FieldName("b")}),
			IneqConditions:          set.From[FieldName]([]FieldName{FieldName("c")}),
			SelectOrOtherConditions: set.From[FieldName]([]FieldName{FieldName("d"), FieldName("e")}),
		}

		idxFamily := qFamily.ToIndexFamilies()

		idxFamily.ForEach(func(family IndexFamily) bool {
			fmt.Printf("%+v\n", family)
			return true
		})
		asserts.Equal(1, idxFamily.Size(), "Just one possible inequality condition, so 1 family is rendered")
		expected := set.HashSetFrom[IndexFamily]([]IndexFamily{
			{
				Fixed:    []FieldName{},
				Flex:     set.From([]FieldName{FieldName("a"), FieldName("b")}),
				Last:     FieldName("c"),
				Included: set.From[FieldName]([]FieldName{FieldName("d"), FieldName("e")}),
			},
		})
		asserts.True(expected.Equal(idxFamily), "The index families in the result are the expected ones")
	})

	t.Run("Case without an inequality condition", func(t *testing.T) {
		asserts := assert.New(t)

		qFamily := AggregateQueryFamily{
			TableName:               "",
			EqConditions:            set.From[FieldName]([]FieldName{FieldName("a"), FieldName("b")}),
			IneqConditions:          set.New[FieldName](0),
			SelectOrOtherConditions: set.From[FieldName]([]FieldName{FieldName("d"), FieldName("e")}),
		}

		idxFamily := qFamily.ToIndexFamilies()

		idxFamily.ForEach(func(family IndexFamily) bool {
			fmt.Printf("%+v\n", family)
			return true
		})
		asserts.Equal(1, idxFamily.Size(), "No inequality condition, so 1 family is rendered")
		expected := set.HashSetFrom[IndexFamily]([]IndexFamily{
			{
				Fixed:    []FieldName{},
				Flex:     set.From([]FieldName{FieldName("a"), FieldName("b")}),
				Last:     FieldName(""),
				Included: set.From[FieldName]([]FieldName{FieldName("d"), FieldName("e")}),
			},
		})
		asserts.True(expected.Equal(idxFamily), "The index families in the result are the expected ones")
	})
}
