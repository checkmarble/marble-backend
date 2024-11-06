package models

import "fmt"

type DatabaseSchemaType int

const (
	// Marble Database schema
	DATABASE_SCHEMA_TYPE_MARBLE DatabaseSchemaType = iota
	// client's shema database
	DATABASE_SCHEMA_TYPE_CLIENT
)

type DatabaseSchema struct {
	SchemaType DatabaseSchemaType
	Schema     string
}

var DATABASE_MARBLE_SCHEMA = DatabaseSchema{
	SchemaType: DATABASE_SCHEMA_TYPE_MARBLE,
	Schema:     "marble",
}

type OrganizationSchema struct {
	OrganizationId string
	DatabaseSchema DatabaseSchema
}

func OrgSchemaName(orgName string) string {
	return fmt.Sprintf("org-%s", orgName)
}
