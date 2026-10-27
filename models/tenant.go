package models

import "github.com/google/uuid"

type Tenant struct {
	Id   uuid.UUID `json:"id"`
	Name string    `json:"name"`
	// OIDCConfiguration is nil when the tenant has no OIDC configuration. It is
	// loaded without the client secret.
	OIDCConfiguration *TenantOIDCConfiguration `json:"-"`
}
