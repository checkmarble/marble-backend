package api

import (
	"context"
	"testing"

	"github.com/checkmarble/marble-backend/infra"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/usecases/auth"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestConfigAuthProviders(t *testing.T) {
	registry := infra.NewTenantOIDCRegistry(func(context.Context, uuid.UUID) (models.TenantOIDCConfiguration, error) {
		return models.TenantOIDCConfiguration{}, nil
	}, "http://localhost/oidc/callback", true)

	tests := []struct {
		name   string
		config Configuration
		want   []string
	}{
		{name: "none", want: []string{}},
		{name: "firebase", config: Configuration{TokenProviders: auth.TokenProviders{auth.TokenProviderFirebase}}, want: []string{"firebase"}},
		{name: "firebase + oidc", config: Configuration{TokenProviders: auth.TokenProviders{auth.TokenProviderFirebase}, TenantOIDCRegistry: registry}, want: []string{"firebase", "oidc"}},
		{name: "legacy", config: Configuration{TokenProviders: auth.TokenProviders{auth.TokenProviderOidc}}, want: []string{"oidc-legacy"}},
		{name: "tenant", config: Configuration{TenantOIDCRegistry: registry}, want: []string{"oidc"}},
		{name: "all", config: Configuration{TokenProviders: auth.TokenProviders{auth.TokenProviderFirebase, auth.TokenProviderOidc}, TenantOIDCRegistry: registry}, want: []string{"firebase", "oidc-legacy", "oidc"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, configAuthProviders(tt.config))
		})
	}
}
