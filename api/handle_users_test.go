package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetCredentialsSections(t *testing.T) {
	creds := models.Credentials{RoleBindings: models.NativeRoleBindings([]models.Role{models.ANALYST})}
	ctx := context.WithValue(context.Background(), utils.ContextKeyCredentials, creds)
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request = httptest.NewRequest(http.MethodGet, "/credentials", nil).WithContext(ctx)

	handleGetCredentials()(c)

	require.Equal(t, http.StatusOK, response.Code)

	var body struct {
		Sections map[string]bool `json:"sections"`
	}

	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &body))
	assert.False(t, body.Sections["settings"])
	assert.True(t, body.Sections["case_manager"])
}
