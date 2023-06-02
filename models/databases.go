package models

// Mable backend connects to multiple databases simultaneously.
// - One instance of Marble database (hardcoded).
// - Multiple intances of client's databases.
type DatabaseType int

const (
	DATABASE_TYPE_INVALID DatabaseType = iota
	// Marble database: there isone just one in practice
	DATABASE_TYPE_MARBLE
	// client's database
	DATABASE_TYPE_CLIENT
)

type PostgresConnection string

type Database struct {
	DatabaseType DatabaseType
	Connection   PostgresConnection
}

// There is only one instance of Marble database
var DATABASE_MARBLE = Database{
	DatabaseType: DATABASE_TYPE_MARBLE,
	Connection:   PostgresConnection("connection string to marble database"),
}
