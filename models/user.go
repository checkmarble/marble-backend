package models

import (
	"time"

	"github.com/google/uuid"
)

type UserId string

type User struct {
	UserId            UserId
	Email             string
	Role              Role
	TenantId          uuid.UUID
	OrganizationIds   []uuid.UUID // Active direct organization memberships, not identity ownership.
	OrganizationRoles map[uuid.UUID][]Role
	FirstName         string
	LastName          string
	DeletedAt         *time.Time

	// Currently only used to control display of the AI assist button in the UI - DO NOT use for anything else as it will be removed
	AiAssistEnabled bool
	Picture         string

	// TfaEnabled reflects whether the user has at least one multi-factor
	// authentication factor enrolled with the identity provider. It is only
	// populated on demand (it requires an identity provider round-trip) and is
	// nil when not computed, so a nil value must not be read as "no MFA".
	TfaEnabled *bool
}

func (u User) FullName() string {
	if u.FirstName == "" && u.LastName == "" {
		return ""
	}
	return u.FirstName + " " + u.LastName
}

type CreateUser struct {
	TenantId       uuid.UUID
	Email          string
	Role           Role
	OrganizationId uuid.UUID
	FirstName      string
	LastName       string
}

type UpdateUser struct {
	OrganizationId uuid.UUID
	UserId         string
	Email          *string
	Role           *Role
	FirstName      *string
	LastName       *string
}
