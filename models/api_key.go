package models

import "time"

type ApiKey struct {
	Id             string
	CreatedAt      time.Time
	Description    string
	Hash           []byte
	OrganizationId string
	PartnerId      string
	Prefix         string
	Role           Role
}

type CreateApiKeyInput struct {
	Description    string
	OrganizationId string
	Role           Role
}

type CreatedApiKey struct {
	ApiKey
	Key string
}
