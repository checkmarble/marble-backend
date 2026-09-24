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
	// TODO(MAR-2251): deprecated authorization field; use grants after legacy JWT expiry.
	Role Role

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
