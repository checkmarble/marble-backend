package models

type ApiKey struct {
	Id             string
	OrganizationId string
	Hash           string
	Description    string
	Role           Role
}

type CreatedApiKey struct {
	ApiKey
	Value string
}

type CreateApiKeyInput struct {
	OrganizationId string
	Description    string
	Role           Role
}

type CreateApiKey struct {
	CreateApiKeyInput
	Id   string
	Hash string
}
