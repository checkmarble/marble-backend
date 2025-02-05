package integration

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/gavv/httpexpect/v2"
)

func TestApiEndToEnd(t *testing.T) {
	e := httpexpect.Default(t, testServer.URL)

	e.GET("/liveness").Expect().Status(http.StatusOK)

	// Create an org and an admin user in it
	authOrgAdmin, authOrgViewer := setupOrgAndUser(e)

	// create the data model
	setupDataModel(authOrgAdmin, authOrgViewer)

	// create a scenario and publish it
	scenarioId := setupTestScenarioAndPublish(authOrgAdmin, authOrgViewer)

	// create an api key
	authApiKey := setupApiKey(e, authOrgAdmin)

	// ingest data and call a decision
	ingestAndCreateDecision(authApiKey, scenarioId)
}

func setupOrgAndUser(e *httpexpect.Expect) (authOrgAdmin *httpexpect.Expect, authOrgViewer *httpexpect.Expect) {
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
	orgId := auth.POST("/organizations").WithJSON(map[string]any{"name": "test-org"}).
		Expect().Status(http.StatusOK).
		JSON().
		Object().Value("organization").
		Object().Value("id").String().NotEmpty().Raw()

	orgAdminEmail := "test@email.com"
	auth.POST("/users").
		WithJSON(map[string]any{"email": orgAdminEmail, "organization_id": orgId, "role": "ADMIN"}).
		Expect().Status(http.StatusOK).
		JSON().
		Object().Value("user").
		Object().Value("user_id").String().NotEmpty().Raw()

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

	authOrgAdmin = e.Builder(func(req *httpexpect.Request) {
		req.WithHeader("Authorization", fmt.Sprintf("Bearer %s", orgAdminToken))
	})

	// org admin cannot create Marble admin
	authOrgAdmin.POST("/users").
		WithJSON(map[string]any{"email": "reject@reject.com", "role": "MARBLE_ADMIN"}).
		Expect().Status(http.StatusForbidden)

	// create a viewer user
	viewerEmail := "viewer@email.com"
	authOrgAdmin.POST("/users").
		WithJSON(map[string]any{"email": viewerEmail, "organization_id": orgId, "role": "VIEWER"}).
		Expect().Status(http.StatusOK)
	orgViewerToken := e.POST("/token").
		WithHeader("Authorization", fmt.Sprintf("Bearer %s", viewerEmail)).
		Expect().Status(http.StatusOK).
		JSON().
		Object().ContainsKey("access_token").
		Value("access_token").String().Raw()

	authOrgViewer = e.Builder(func(req *httpexpect.Request) {
		req.WithHeader("Authorization", fmt.Sprintf("Bearer %s", orgViewerToken))
	})

	return authOrgAdmin, authOrgViewer
}

func setupDataModel(auth *httpexpect.Expect, authOrgViewer *httpexpect.Expect) {
	authOrgViewer.POST("/data-model/tables").
		WithJSON(map[string]any{"name": "transactions", "description": "the transactions table"}).
		Expect().Status(http.StatusForbidden)

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
	linkId := authOrgViewer.GET("/data-model").
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

func setupTestScenarioAndPublish(authOrgAdmin *httpexpect.Expect, authOrgViewer *httpexpect.Expect) string {
	scenarioId := authOrgAdmin.POST("/scenarios").
		WithJSON(map[string]any{"name": "test-scenario", "trigger_object_type": "transactions"}).
		Expect().Status(http.StatusOK).
		JSON().
		Object().Value("id").String().NotEmpty().Raw()

	scenarioIterationId := authOrgAdmin.POST("/scenario-iterations").
		WithJSON(map[string]any{"scenario_id": scenarioId}).
		Expect().Status(http.StatusOK).
		JSON().
		Object().Value("id").String().NotEmpty().Raw()

	triggerBody := `{
  "body": {
    "score_review_threshold": 10,
	"score_block_and_review_threshold": 10,
    "score_decline_threshold": 20,
    "trigger_condition_ast_expression": {
      "name": "And",
      "children": [
        {
          "name": "=",
          "children": [
            { "name": "Payload", "children": [{ "constant": "status" }] },
            { "constant": "validated" }
          ]
        }
      ]
    }
  }
}
`
	authOrgAdmin.PATCH("/scenario-iterations/{iteration_id}", scenarioIterationId).
		WithBytes([]byte(triggerBody)).
		Expect().Status(http.StatusOK)

	ruleBody := fmt.Sprintf(`{
  "scenario_iteration_id": "%s",
  "name": "Test rule 1",
  "formula_ast_expression": {
    "name": "And",
    "children": [
      {
        "name": "=",
        "children": [
          { "name": "Payload", "children": [{ "constant": "status" }] },
          { "constant": "validated" }
        ]
      }
    ]
  },
  "score_modifier": 2
}`, scenarioIterationId)

	authOrgAdmin.POST("/scenario-iteration-rules").WithBytes([]byte(ruleBody)).
		Expect().Status(http.StatusOK)

	// validate the scenario
	authOrgViewer.POST("/scenario-iterations/{iteration_id}/validate", scenarioIterationId).
		Expect().Status(http.StatusOK)

	// commit the scenario
	authOrgViewer.POST("/scenario-iterations/{iteration_id}/commit", scenarioIterationId).
		Expect().Status(http.StatusForbidden)

	authOrgAdmin.POST("/scenario-iterations/{iteration_id}/commit", scenarioIterationId).
		Expect().Status(http.StatusOK)

	// activate the scenario
	authOrgAdmin.POST("/scenario-publications").
		WithJSON(map[string]any{"scenario_iteration_id": scenarioIterationId, "publication_action": "publish"}).
		Expect().Status(http.StatusOK)

	return scenarioId
}

func setupApiKey(e *httpexpect.Expect, authOrgAdmin *httpexpect.Expect) *httpexpect.Expect {
	apiKey := authOrgAdmin.POST("/apikeys").
		WithJSON(map[string]any{"role": "API_CLIENT", "description": "test api key"}).
		Expect().Status(http.StatusCreated).
		JSON().
		Object().Value("api_key").
		Object().Value("key").String().NotEmpty().Raw()

	auth := e.Builder(func(req *httpexpect.Request) {
		req.WithHeader("x-api-key", apiKey)
	})
	return auth
}

func ingestAndCreateDecision(authApiKey *httpexpect.Expect, scenarioId string) {
	authApiKey.POST("/ingestion/transactions").
		WithBytes([]byte(`{
  "object_id": "my-unique-id",
  "updated_at": "2024-01-01T00:00:00Z",
  "account_id": "my-account-id",
  "amount": 100,
  "status": "validated",
  "transaction_at": "2024-01-01T00:00:00Z"
}`)).
		Expect().Status(http.StatusCreated)

	// also do some batch ingestion
	authApiKey.POST("/ingestion/transactions/multiple").
		WithBytes([]byte(`
		[
			{
				"object_id": "my-unique-id",
				"updated_at": "2024-01-01T00:00:00Z",
				"account_id": "my-account-id",
				"amount": 100,
				"status": "validated",
				"transaction_at": "2024-01-01T00:00:00Z"
			},
			{
				"object_id": "my-unique-id-2",
				"updated_at": "2024-01-01T00:00:00Z",
				"account_id": "my-account-id-2",
				"amount": 100,
				"status": "validated",
				"transaction_at": "2024-01-01T00:00:00Z"
			}
		]`)).
		Expect().Status(http.StatusCreated)

	objectMap := map[string]any{
		"object_id":      "my-unique-id",
		"updated_at":     "2024-01-01T00:00:00Z",
		"account_id":     "my-account-id",
		"amount":         100,
		"status":         "validated",
		"transaction_at": "2024-01-01T00:00:00Z",
	}
	dec := authApiKey.POST("/decisions").
		WithJSON(map[string]any{"scenario_id": scenarioId, "object_type": "transactions", "trigger_object": objectMap}).
		Expect().Status(http.StatusOK).JSON()
	dec.Object().Value("rules").Array().Length().IsEqual(1)
	dec.Object().Value("outcome").String().IsEqual("approve")
	dec.Object().Value("score").Number().IsEqual(2)
	dec.Object().Value("pivot_values").Array().Length().IsEqual(1)

	authApiKey.POST("/decisions/all").
		WithJSON(map[string]any{
			"object_type":    "transactions",
			"trigger_object": objectMap,
		}).
		Expect().Status(http.StatusOK).JSON().
		Object().Value("decisions").
		Array().Length().IsEqual(1)
}
