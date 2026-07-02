package models

import (
	"time"

	"github.com/google/uuid"
)

type ApiKey struct {
	Id             string
	CreatedAt      time.Time
	Description    string
	Hash           []byte
	OrganizationId uuid.UUID
	Prefix         string
	// Role           Role
	Roles []Role

	DisplayString string
}

type CreateApiKeyInput struct {
	Description    string
	OrganizationId uuid.UUID
	Roles          []Role
}

type CreatedApiKey struct {
	ApiKey
	Key string
}
