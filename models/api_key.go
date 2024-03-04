package models

import "time"

type ApiKey struct {
	Id             string
	CreatedAt      time.Time
	Description    string
	Key            string
	Hash           []byte
	OrganizationId string
	Role           Role
}

type CreateApiKeyInput struct {
	Description    string
	OrganizationId string
	Role           Role
}
