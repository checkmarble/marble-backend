package models

type ApiKey struct {
	Id             string
	OrganizationId string
	Key            string
	Description    string
	Role           Role
}

type CreateApiKeyInput struct {
	OrganizationId string
	Key            string
	Description    string
}
