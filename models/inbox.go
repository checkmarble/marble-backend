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
	CasesCount     *int
}

type CreateInboxInput struct {
	Name           string
	OrganizationId string
}

type UpdateInboxInput struct {
	Id   string
	Name string
}
