package models

// Mable backend connects to multiple databases simultaneously.
// - One instance of Marble database (hardcoded).
// - Multiple intances of client's databases.
type DatabaseType int

const (
	// Marble database: there isone just one in practice
	DATABASE_TYPE_MARBLE DatabaseType = iota
	// client's database
	DATABASE_TYPE_CLIENT
)

type DatabaseName string

type Database struct {
	DatabaseType DatabaseType
	Name         DatabaseName
}

// There is only one instance of Marble database
var DATABASE_MARBLE = Database{
	DatabaseType: DATABASE_TYPE_MARBLE,
	Name:         DatabaseName("marble"),
}
