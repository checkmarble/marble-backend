package api

import (
	"context"
	"log/slog"
	"testing"

	"github.com/checkmarble/marble-backend/infra"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/usecases"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestAddRoutes_AuthDiscoveryRequiresTenantOIDCRegistry(t *testing.T) {
	registry := infra.NewTenantOIDCRegistry(func(context.Context, uuid.UUID) (models.TenantOIDCConfiguration, error) {
		return models.TenantOIDCConfiguration{}, nil
	}, "https://app.example.test/oidc/callback", false)

	tests := []struct {
		name             string
		configuration    Configuration
		tenantOIDCRoutes bool
	}{
		{
			name: "without OIDC",
		},
		{
			name: "with global OIDC only",
			configuration: Configuration{
				OidcConfig: infra.OidcConfig{Provider: &oidc.Provider{}},
			},
		},
		{
			name: "with tenant OIDC",
			configuration: Configuration{
				TenantOIDCRegistry: registry,
			},
			tenantOIDCRoutes: true,
		},
		{
			name: "with global and tenant OIDC",
			configuration: Configuration{
				OidcConfig:         infra.OidcConfig{Provider: &oidc.Provider{}},
				TenantOIDCRegistry: registry,
			},
			tenantOIDCRoutes: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			addRoutes(router, tt.configuration, usecases.Usecases{}, utils.Authentication{}, TokenHandler{}, slog.New(slog.DiscardHandler))

			for _, route := range tenantOIDCRoutes {
				assert.Equal(t, tt.tenantOIDCRoutes, hasRoute(router, route.method, route.path), route.method+" "+route.path)
			}
		})
	}
}

var tenantOIDCRoutes = []struct {
	method string
	path   string
}{
	{method: "POST", path: "/auth/discovery"},
	{method: "GET", path: "/tenants/:tenant_id/oidc-configuration"},
	{method: "PUT", path: "/tenants/:tenant_id/oidc-configuration"},
	{method: "DELETE", path: "/tenants/:tenant_id/oidc-configuration"},
	{method: "GET", path: "/tenants/:tenant_id/domains"},
	{method: "POST", path: "/tenants/:tenant_id/domains"},
	{method: "DELETE", path: "/tenants/:tenant_id/domains/:domain_id"},
}

func hasRoute(router *gin.Engine, method, path string) bool {
	for _, route := range router.Routes() {
		if route.Method == method && route.Path == path {
			return true
		}
	}
	return false
}
