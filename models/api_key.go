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
	PartnerId      *string
	Prefix         string
	Role           Role

	DisplayString string
}

type CreateApiKeyInput struct {
	Description    string
	OrganizationId uuid.UUID
	Role           Role
}

type CreatedApiKey struct {
	ApiKey
	Key string
}
