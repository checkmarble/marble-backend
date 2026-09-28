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
	RoleBindings   []RoleBinding

	DisplayString string
}

type CreateApiKeyInput struct {
	Description    string
	OrganizationId uuid.UUID
	RoleBindings   []RoleBinding
}

type CreatedApiKey struct {
	ApiKey
	Key string
}
