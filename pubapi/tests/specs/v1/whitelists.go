package v1

import (
	"net/http"
	"testing"

	api "github.com/checkmarble/marble-backend/pubapi/v1"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/gavv/httpexpect/v2"
	"github.com/hashicorp/go-set/v2"
	"gotest.tools/v3/assert"
)

func whitelists(t *testing.T, e *httpexpect.Expect) {
	e.POST("/sanction-checks/whitelists").
		WithJSON(api.AddWhitelistParams{Counterparty: "Jean-Baptiste Zorg", EntityId: "ABC123"}).
		Expect().
		Status(http.StatusCreated)

	e.POST("/sanction-checks/whitelists").
		WithJSON(api.AddWhitelistParams{Counterparty: "Joe Bill", EntityId: "ABC123"}).
		Expect().
		Status(http.StatusCreated)

	e.POST("/sanction-checks/whitelists").
		WithJSON(api.AddWhitelistParams{Counterparty: "JBZ", EntityId: "ABC123"}).
		Expect().
		Status(http.StatusCreated)

	{
		out := e.POST("/sanction-checks/whitelists/search").
			WithJSON(api.SearchWhitelistParams{EntityId: utils.Ptr("ABC123")}).
			Expect().
			Status(http.StatusOK).
			JSON().Path("$.data").Array()

		found := set.New[string](0)

		out.Every(func(_ int, value *httpexpect.Value) {
			value.Object().HasValue("entity_id", "ABC123")

			found.Insert(value.Object().Value("counterparty").InList(
				"Jean-Baptiste Zorg", "JBZ", "Joe Bill").String().Raw())
		})

		assert.Equal(t, 3, found.Size(), "not all counterparties were found matching the entity ID")
	}

	e.DELETE("/sanction-checks/whitelists").
		WithJSON(api.DeleteWhitelistParams{EntityId: "ABC123", Counterparty: utils.Ptr("Joe Bill")}).
		Expect().
		Status(http.StatusNoContent)

	{
		out := e.POST("/sanction-checks/whitelists/search").
			WithJSON(api.SearchWhitelistParams{EntityId: utils.Ptr("ABC123")}).
			Expect().
			Status(http.StatusOK).
			JSON().Path("$.data").Array()

		out.Length().IsEqual(2)
		out.Every(func(i int, value *httpexpect.Value) {
			value.Object().NotHasValue("counterparty", "Joe Bill")
		})
	}

	e.DELETE("/sanction-checks/whitelists").
		WithJSON(api.DeleteWhitelistParams{EntityId: "ABC123"}).
		Expect().
		Status(http.StatusNoContent)

	e.POST("/sanction-checks/whitelists/search").
		WithJSON(api.SearchWhitelistParams{EntityId: utils.Ptr("ABC123")}).
		Expect().
		Status(http.StatusOK).
		JSON().Path("$.data").Array().
		Length().IsEqual(0)
}
