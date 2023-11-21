package models

import "time"

type InboxStatus string

const (
	InboxStatusActive   InboxStatus = "active"
	InboxStatusInactive InboxStatus = "archived"
)

type Inbox struct {
	Id             string
	Name           string
	OrganizationId string
	Status         InboxStatus
	CreatedAt      time.Time
	UpdatedAt      time.Time
	InboxUsers     []InboxUser
}

type CreateInboxInput struct {
	Name           string
	OrganizationId string
}

type InboxUser struct {
	Id        string
	InboxId   string
	UserId    string
	CreatedAt time.Time
	UpdatedAt time.Time
	Role      InboxUserRole
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
