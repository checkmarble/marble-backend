package models

import "time"

type InboxStatus string

const (
	InboxStatusActive   InboxStatus = "active"
	InboxStatusInactive InboxStatus = "archived"
)

type Inbox struct {
	Id                string
	Name              string
	OrganizationId    string
	Status            InboxStatus
	EscalationInboxId *string
	CreatedAt         time.Time
	UpdatedAt         time.Time
	InboxUsers        []InboxUser
	CasesCount        *int
}

type InboxMetadata struct {
	Id     string
	Name   string
	Status InboxStatus
}

func (i Inbox) GetMetadata() InboxMetadata {
	return InboxMetadata{
		Id:     i.Id,
		Name:   i.Name,
		Status: i.Status,
	}
}

type CreateInboxInput struct {
	Name              string
	OrganizationId    string
	EscalationInboxId *string
}

type UpdateInboxInput struct {
	Id                string
	Name              string
	EscalationInboxId *string
}
