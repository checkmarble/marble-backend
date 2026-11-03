package api

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestOrganizationIDForGrant(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tenant, org := uuid.New(), uuid.New()
	for _, tt := range []struct {
		name    string
		creds   models.Credentials
		allowed bool
	}{
		{name: "tenant admin", creds: models.Credentials{TenantId: tenant, Roles: []models.Role{models.TENANT_ADMIN}}, allowed: true},
		{name: "another tenant", creds: models.Credentials{TenantId: uuid.New(), Roles: []models.Role{models.TENANT_ADMIN}}},
		{name: "organization admin without access", creds: models.Credentials{Roles: []models.Role{models.ADMIN}}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest("PUT", "/?organization-id="+org.String(), nil)
			c.Request = c.Request.WithContext(context.WithValue(c.Request.Context(), utils.ContextKeyCredentials, tt.creds))
			got, err := organizationIDForGrant(c, tenant)
			if tt.allowed {
				require.NoError(t, err)
				require.Equal(t, org, got)
			} else {
				require.Error(t, err)
			}
		})
	}
}
