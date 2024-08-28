package integration

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/gavv/httpexpect/v2"
	"github.com/stretchr/testify/assert"
)

func TestApiEndToEnd(t *testing.T) {
	e := httpexpect.Default(t, fmt.Sprintf("http://localhost:%s", port))

	e.GET("/liveness").Expect().Status(http.StatusOK)

	// Get a token as a marble admin
	adminToken := e.POST("/token").
		WithHeader("Authorization", fmt.Sprintf("Bearer %s", marbleAdminEmail)).
		Expect().Status(http.StatusOK).
		JSON().
		Object().ContainsKey("access_token").
		Value("access_token").String().Raw()
	assert.NotEqual(t, adminToken, "")

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

	userEmail := "test@email.com"
	userId := auth.POST("/users").
		WithJSON(map[string]any{"email": userEmail, "organization_id": orgId, "role": "ADMIN"}).
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
}
