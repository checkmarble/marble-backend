package models

type ApiKeyId string

type ApiKey struct {
	ApiKeyId       ApiKeyId
	OrganizationId string
	Key            string
	Role           Role
}
