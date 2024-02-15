package indexes

import (
	"fmt"
	"testing"

	"github.com/checkmarble/marble-backend/models"
	"github.com/hashicorp/go-set/v2"
	"github.com/stretchr/testify/assert"
)

func TestAggregateQueryToIndexFamily(t *testing.T) {
	t.Run("Case with 2 inequality conditions", func(t *testing.T) {
		asserts := assert.New(t)

		qFamily := models.AggregateQueryFamily{
			TableName: "",
			EqConditions: set.From[models.FieldName]([]models.FieldName{
				models.FieldName("a"), models.FieldName("b"),
			}),
			IneqConditions: set.From[models.FieldName]([]models.FieldName{
				models.FieldName("c"), models.FieldName("d"),
			}),
			SelectOrOtherConditions: set.From[models.FieldName]([]models.FieldName{
				models.FieldName("e"), models.FieldName("f"),
			}),
		}

		idxFamily := qFamily.ToIndexFamilies()
		asserts.Equal(2, idxFamily.Size(), "Two possible inequality conditions, so 2 families are rendered")
		expected := set.HashSetFrom[models.IndexFamily]([]models.IndexFamily{
			{
				Fixed: []models.FieldName{},
				Flex:  set.From([]models.FieldName{models.FieldName("a"), models.FieldName("b")}),
				Last:  models.FieldName("c"),
				Included: set.From[models.FieldName]([]models.FieldName{
					models.FieldName("d"),
					models.FieldName("e"), models.FieldName("f"),
				}),
			},
			{
				Fixed: []models.FieldName{},
				Flex:  set.From([]models.FieldName{models.FieldName("a"), models.FieldName("b")}),
				Last:  models.FieldName("d"),
				Included: set.From[models.FieldName]([]models.FieldName{
					models.FieldName("c"),
					models.FieldName("e"), models.FieldName("f"),
				}),
			},
		})
		asserts.True(expected.Equal(idxFamily), "The index families in the result are the expected ones")
	})

	t.Run("Case with 1 inequality conditions", func(t *testing.T) {
		asserts := assert.New(t)

		qFamily := models.AggregateQueryFamily{
			TableName: "",
			EqConditions: set.From[models.FieldName]([]models.FieldName{
				models.FieldName("a"), models.FieldName("b"),
			}),
			IneqConditions: set.From[models.FieldName]([]models.FieldName{
				models.FieldName("c"),
			}),
			SelectOrOtherConditions: set.From[models.FieldName]([]models.FieldName{
				models.FieldName("d"), models.FieldName("e"),
			}),
		}

		idxFamily := qFamily.ToIndexFamilies()

		idxFamily.ForEach(func(family models.IndexFamily) bool {
			fmt.Printf("%+v\n", family)
			return true
		})
		asserts.Equal(1, idxFamily.Size(), "Just one possible inequality condition, so 1 family is rendered")
		expected := set.HashSetFrom[models.IndexFamily]([]models.IndexFamily{
			{
				Fixed:    []models.FieldName{},
				Flex:     set.From([]models.FieldName{models.FieldName("a"), models.FieldName("b")}),
				Last:     models.FieldName("c"),
				Included: set.From[models.FieldName]([]models.FieldName{models.FieldName("d"), models.FieldName("e")}),
			},
		})
		asserts.True(expected.Equal(idxFamily), "The index families in the result are the expected ones")
	})

	t.Run("Case without an inequality condition", func(t *testing.T) {
		asserts := assert.New(t)

		qFamily := models.AggregateQueryFamily{
			TableName: "",
			EqConditions: set.From[models.FieldName]([]models.FieldName{
				models.FieldName("a"), models.FieldName("b"),
			}),
			IneqConditions: set.New[models.FieldName](0),
			SelectOrOtherConditions: set.From[models.FieldName]([]models.FieldName{
				models.FieldName("d"), models.FieldName("e"),
			}),
		}

		idxFamily := qFamily.ToIndexFamilies()

		idxFamily.ForEach(func(family models.IndexFamily) bool {
			fmt.Printf("%+v\n", family)
			return true
		})
		asserts.Equal(1, idxFamily.Size(), "No inequality condition, so 1 family is rendered")
		expected := set.HashSetFrom[models.IndexFamily]([]models.IndexFamily{
			{
				Fixed:    []models.FieldName{},
				Flex:     set.From([]models.FieldName{models.FieldName("a"), models.FieldName("b")}),
				Last:     models.FieldName(""),
				Included: set.From[models.FieldName]([]models.FieldName{models.FieldName("d"), models.FieldName("e")}),
			},
		})
		asserts.True(expected.Equal(idxFamily), "The index families in the result are the expected ones")
	})
}
