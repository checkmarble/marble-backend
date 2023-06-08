package models

type PostgresConnection string

type Database struct {
	Connection PostgresConnection
}

// SchemaType is use
type DatabaseSchemaType int

const (
	// Marble Database schema
	DATABASE_SCHEMA_TYPE_MARBLE DatabaseSchemaType = iota
	// client's shema database
	DATABASE_SCHEMA_TYPE_CLIENT
)

type DatabaseSchema struct {
	SchemaType DatabaseSchemaType
	Database   Database
	Schema     string
}

// There is only one instance of Marble database
var DATABASE_MARBLE = Database{
	Connection: PostgresConnection("connection string to marble database"),
}

var DATABASE_MARBLE_SCHEMA = DatabaseSchema{
	SchemaType: DATABASE_SCHEMA_TYPE_MARBLE,
	Database:   DATABASE_MARBLE,
	Schema:     "marble",
}
