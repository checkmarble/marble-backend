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
			TableName:               "",
			EqConditions:            set.From[string]([]string{"a", "b"}),
			IneqConditions:          set.From[string]([]string{"c", "d"}),
			SelectOrOtherConditions: set.From[string]([]string{"e", "f"}),
		}

		idxFamily := qFamily.ToIndexFamilies()
		asserts.Equal(2, idxFamily.Size(), "Two possible inequality conditions, so 2 families are rendered")
		expected := set.HashSetFrom[models.IndexFamily]([]models.IndexFamily{
			{
				Fixed:    []string{},
				Flex:     set.From([]string{"a", "b"}),
				Last:     "c",
				Included: set.From[string]([]string{"d", "e", "f"}),
			},
			{
				Fixed:    []string{},
				Flex:     set.From([]string{"a", "b"}),
				Last:     "d",
				Included: set.From[string]([]string{"c", "e", "f"}),
			},
		})
		asserts.True(expected.Equal(idxFamily), "The index families in the result are the expected ones")
	})

	t.Run("Case with 1 inequality conditions", func(t *testing.T) {
		asserts := assert.New(t)

		qFamily := models.AggregateQueryFamily{
			TableName:               "",
			EqConditions:            set.From[string]([]string{"a", "b"}),
			IneqConditions:          set.From[string]([]string{"c"}),
			SelectOrOtherConditions: set.From[string]([]string{"d", "e"}),
		}

		idxFamily := qFamily.ToIndexFamilies()

		idxFamily.ForEach(func(family models.IndexFamily) bool {
			fmt.Printf("%+v\n", family)
			return true
		})
		asserts.Equal(1, idxFamily.Size(), "Just one possible inequality condition, so 1 family is rendered")
		expected := set.HashSetFrom[models.IndexFamily]([]models.IndexFamily{
			{
				Fixed:    []string{},
				Flex:     set.From([]string{"a", "b"}),
				Last:     "c",
				Included: set.From[string]([]string{"d", "e"}),
			},
		})
		asserts.True(expected.Equal(idxFamily), "The index families in the result are the expected ones")
	})

	t.Run("Case without an inequality condition", func(t *testing.T) {
		asserts := assert.New(t)

		qFamily := models.AggregateQueryFamily{
			TableName:               "",
			EqConditions:            set.From[string]([]string{"a", "b"}),
			IneqConditions:          set.New[string](0),
			SelectOrOtherConditions: set.From[string]([]string{"d", "e"}),
		}

		idxFamily := qFamily.ToIndexFamilies()

		idxFamily.ForEach(func(family models.IndexFamily) bool {
			fmt.Printf("%+v\n", family)
			return true
		})
		asserts.Equal(1, idxFamily.Size(), "No inequality condition, so 1 family is rendered")
		expected := set.HashSetFrom[models.IndexFamily]([]models.IndexFamily{
			{
				Fixed:    []string{},
				Flex:     set.From([]string{"a", "b"}),
				Last:     "",
				Included: set.From[string]([]string{"d", "e"}),
			},
		})
		asserts.True(expected.Equal(idxFamily), "The index families in the result are the expected ones")
	})

	t.Run("Case with no conditions", func(t *testing.T) {
		asserts := assert.New(t)

		qFamily := models.NewAggregateQueryFamily("")
		qFamily.SelectOrOtherConditions = set.From([]string{"a", "b", "c"})

		idxFamily := qFamily.ToIndexFamilies()
		asserts.Equal(0, idxFamily.Size(), "No indexable condition, so no family is rendered")
	})
}
