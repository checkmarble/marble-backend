package models

import (
	"time"

	"github.com/google/uuid"
)

// TenantOIDCConfiguration is deliberately explicit about the secret: it is
// populated only by the repository authentication read path and never exposed
// by an API DTO.
type TenantOIDCConfiguration struct {
	TenantId       uuid.UUID
	Issuer         string
	ClientId       string
	ClientSecret   string
	Scopes         []string
	ExtraParams    map[string]string
	EmailClaim     string
	SsoEnabled     bool
	CreatedAt      time.Time
	UpdatedAt      time.Time
	AllowedDomains []string
}

type TenantDomain struct {
	Id         uuid.UUID
	TenantId   uuid.UUID
	Domain     string
	VerifiedAt time.Time
	CreatedAt  time.Time
}
