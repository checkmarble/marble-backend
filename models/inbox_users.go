package models

import "time"

type InboxUser struct {
	Id             string
	InboxId        string
	UserId         string
	OrganizationId string
	CreatedAt      time.Time
	UpdatedAt      time.Time
	Role           InboxUserRole
}

type CreateInboxUserInput struct {
	InboxId string
	UserId  string
	Role    InboxUserRole
}

type InboxUserRole string

const (
	InboxUserRoleAdmin  InboxUserRole = "admin"
	InboxUserRoleMember InboxUserRole = "member"
)

type InboxUserFilterInput struct {
	InboxId string
	UserId  UserId
}
