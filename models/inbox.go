package models

import (
	"time"

	"github.com/google/uuid"
)

type InboxStatus string

const (
	InboxStatusActive   InboxStatus = "active"
	InboxStatusInactive InboxStatus = "archived"
)

type Inbox struct {
	Id                uuid.UUID
	Name              string
	OrganizationId    string
	Status            InboxStatus
	EscalationInboxId *uuid.UUID
	CreatedAt         time.Time
	UpdatedAt         time.Time
	InboxUsers        []InboxUser
	CasesCount        *int
}

type InboxMetadata struct {
	Id     uuid.UUID
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
	EscalationInboxId *uuid.UUID
}

type UpdateInboxInput struct {
	Id                uuid.UUID
	Name              string
	EscalationInboxId *uuid.UUID
}
