package models

import (
	"time"

	"github.com/google/uuid"
)

type InboxUser struct {
	Id             uuid.UUID
	InboxId        uuid.UUID
	UserId         uuid.UUID
	OrganizationId string
	CreatedAt      time.Time
	UpdatedAt      time.Time
	Role           InboxUserRole
	AutoAssignable bool
}

type CreateInboxUserInput struct {
	InboxId uuid.UUID
	UserId  uuid.UUID
	Role    InboxUserRole
}

type InboxUserRole string

const (
	InboxUserRoleAdmin  InboxUserRole = "admin"
	InboxUserRoleMember InboxUserRole = "member"
)

type InboxUserFilterInput struct {
	InboxId uuid.UUID
	UserId  UserId
}
