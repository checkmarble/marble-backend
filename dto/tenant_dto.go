package dto

import (
	"github.com/checkmarble/marble-backend/models"
	"github.com/google/uuid"
)

type Tenant struct {
	Id                uuid.UUID                `json:"id"`
	Name              string                   `json:"name"`
	OIDCConfiguration *TenantOIDCConfiguration `json:"oidc_configuration"`
}

func AdaptTenant(tenant models.Tenant) Tenant {
	out := Tenant{Id: tenant.Id, Name: tenant.Name}
	if tenant.OIDCConfiguration != nil {
		config := AdaptTenantOIDCConfiguration(*tenant.OIDCConfiguration)
		out.OIDCConfiguration = &config
	}
	return out
}

// TenantOIDCConfiguration never carries the client secret, only whether one is
// set. A stored configuration always has one.
type TenantOIDCConfiguration struct {
	TenantId        uuid.UUID         `json:"tenant_id"`
	Issuer          string            `json:"issuer"`
	ClientId        string            `json:"client_id"`
	ClientSecretSet bool              `json:"client_secret_set"`
	Scopes          []string          `json:"scopes"`
	ExtraParams     map[string]string `json:"extra_params"`
	EmailClaim      string            `json:"email_claim"`
	SsoEnabled      bool              `json:"sso_enabled"`
}

func AdaptTenantOIDCConfiguration(config models.TenantOIDCConfiguration) TenantOIDCConfiguration {
	return TenantOIDCConfiguration{
		TenantId: config.TenantId, Issuer: config.Issuer, ClientId: config.ClientId, ClientSecretSet: true,
		Scopes: config.Scopes, ExtraParams: config.ExtraParams, EmailClaim: config.EmailClaim, SsoEnabled: config.SsoEnabled,
	}
}
