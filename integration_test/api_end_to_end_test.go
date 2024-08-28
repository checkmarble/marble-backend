package integration

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/gavv/httpexpect/v2"
)

func TestApiEndToEnd(t *testing.T) {
	e := httpexpect.Default(t, fmt.Sprintf("http://localhost:%s", port))

	e.GET("/liveness").Expect().Status(http.StatusOK)

	// Create an org and an admin user in it
	auth, _, _ := setupOrgAndUser(e)

	// create the data model
	setupDataModel(auth)
}

func setupOrgAndUser(e *httpexpect.Expect) (authUser *httpexpect.Expect, orgId, userId string) {
	adminToken := e.POST("/token").
		WithHeader("Authorization", fmt.Sprintf("Bearer %s", marbleAdminEmail)).
		Expect().Status(http.StatusOK).
		JSON().
		Object().ContainsKey("access_token").
		Value("access_token").String().NotEmpty().Raw()

	// New expect instance with marble admin credentials
	auth := e.Builder(func(req *httpexpect.Request) {
		req.WithHeader("Authorization", fmt.Sprintf("Bearer %s", adminToken))
	})

	// Check that there only one user
	obj := auth.GET("/users").
		Expect().Status(http.StatusOK).
		JSON().Object()
	obj.Keys().ContainsOnly("users")
	obj.Value("users").Array().Length().IsEqual(1)

	// Create an org and an admin user in it
	orgId = auth.POST("/organizations").WithJSON(map[string]any{"name": "test-org"}).
		Expect().Status(http.StatusOK).
		JSON().
		Object().Value("organization").
		Object().Value("id").String().NotEmpty().Raw()

	orgAdminEmail := "test@email.com"
	userId = auth.POST("/users").
		WithJSON(map[string]any{"email": orgAdminEmail, "organization_id": orgId, "role": "ADMIN"}).
		Expect().Status(http.StatusOK).
		JSON().
		Object().Value("user").
		Object().Value("user_id").String().NotEmpty().Raw()
	fmt.Println(userId)

	// Check that there are now 2 users
	obj = auth.GET("/users").
		Expect().Status(http.StatusOK).
		JSON().Object()
	obj.Keys().ContainsOnly("users")
	obj.Value("users").Array().Length().IsEqual(2)
	// but only one user in the org
	obj = auth.GET("/organizations/{org_id}/users", orgId).
		Expect().Status(http.StatusOK).
		JSON().Object()
	obj.Keys().ContainsOnly("users")
	obj.Value("users").Array().Length().IsEqual(1)

	// Build an authenticated client as the org's admin user
	orgAdminToken := e.POST("/token").
		WithHeader("Authorization", fmt.Sprintf("Bearer %s", orgAdminEmail)).
		Expect().Status(http.StatusOK).
		JSON().
		Object().ContainsKey("access_token").
		Value("access_token").String().Raw()

	authUser = e.Builder(func(req *httpexpect.Request) {
		req.WithHeader("Authorization", fmt.Sprintf("Bearer %s", orgAdminToken))
	})

	return authUser, orgId, userId
}

func setupDataModel(auth *httpexpect.Expect) {
	transactionsTableId := auth.POST("/data-model/tables").
		WithJSON(map[string]any{"name": "transactions", "description": "the transactions table"}).
		Expect().Status(http.StatusOK).
		JSON().Object().
		Value("id").String().NotEmpty().Raw()

	accountsTableId := auth.POST("/data-model/tables").
		WithJSON(map[string]any{"name": "accounts", "description": "the accounts table"}).
		Expect().Status(http.StatusOK).
		JSON().Object().
		Value("id").String().NotEmpty().Raw()

	auth.POST("/data-model/tables/{table_id}/fields", transactionsTableId).
		WithJSON(map[string]any{"name": "amount", "type": "Float"}).
		Expect().Status(http.StatusOK).
		JSON().Object().Value("id").String().NotEmpty()
	accountIdFieldId := auth.POST("/data-model/tables/{table_id}/fields", transactionsTableId).
		WithJSON(map[string]any{"name": "account_id", "type": "String"}).
		Expect().Status(http.StatusOK).
		JSON().Object().Value("id").String().NotEmpty().Raw()
	auth.POST("/data-model/tables/{table_id}/fields", transactionsTableId).
		WithJSON(map[string]any{"name": "status", "type": "String"}).
		Expect().Status(http.StatusOK).
		JSON().Object().Value("id").String().NotEmpty()
	auth.POST("/data-model/tables/{table_id}/fields", transactionsTableId).
		WithJSON(map[string]any{"name": "transaction_at", "type": "Timestamp"}).
		Expect().Status(http.StatusOK).
		JSON().Object().Value("id").String().NotEmpty()

	auth.POST("/data-model/tables/{table_id}/fields", accountsTableId).
		WithJSON(map[string]any{"name": "status", "type": "String"}).
		Expect().Status(http.StatusOK).
		JSON().Object().Value("id").String().NotEmpty()
	accountIdParentFieldId := auth.POST("/data-model/tables/{table_id}/fields", accountsTableId).
		WithJSON(map[string]any{"name": "account_id", "type": "String", "is_unique": true}).
		Expect().Status(http.StatusOK).
		JSON().Object().Value("id").String().NotEmpty().Raw()

	// create a link between the tables
	auth.POST("/data-model/links").
		WithJSON(map[string]any{
			"name":            "account",
			"parent_table_id": accountsTableId,
			"child_table_id":  transactionsTableId,
			"parent_field_id": accountIdParentFieldId,
			"child_field_id":  accountIdFieldId,
		}).
		Expect().Status(http.StatusNoContent)

	// Read the data model to get the link id
	linkId := auth.GET("/data-model").
		Expect().Status(http.StatusOK).
		JSON().
		Object().Value("data_model").
		Object().Value("tables").
		Object().Value("transactions").
		Object().Value("links_to_single").
		Object().Value("account").
		Object().Value("id").String().NotEmpty().
		Raw()

	// Finally, create a pivot value
	auth.POST("/data-model/pivots").
		WithJSON(map[string]any{
			"base_table_id": transactionsTableId,
			"path_link_ids": []string{linkId},
		}).
		Expect().Status(http.StatusOK)
}
