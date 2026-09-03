package models

import "github.com/google/uuid"

type Grant struct {
	Role           Role
	TenantId       uuid.UUID
	OrganizationId uuid.UUID
}

type OrganizationMembership struct {
	Organization Organization
	Roles        []Role
}
