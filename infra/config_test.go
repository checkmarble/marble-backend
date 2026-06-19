package infra

import (
	"testing"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewPgConfigDefaults(t *testing.T) {
	config, err := NewPgConfig()

	require.NoError(t, err)
	assert.Equal(t, "marble", config.Database)
	assert.Equal(t, "5432", config.Port)
	assert.Equal(t, "prefer", config.SslMode)
	assert.Equal(t, DEFAULT_MAX_CONNECTIONS, config.MaxPoolConnections)
	assert.Equal(t, "", config.ClientDbConfigFile)
	assert.Equal(t, "", config.ImpersonateRole)
}

func TestNewPgConfigConnectionString(t *testing.T) {
	t.Setenv("PG_CONNECTION_STRING", "postgres://user:pass@localhost:5432/dbname")

	config, err := NewPgConfig()

	require.NoError(t, err)
	require.NotEmpty(t, config.ConnectionString)
}

func TestNewPgConfigInvalidConnectionString(t *testing.T) {
	t.Setenv("PG_CONNECTION_STRING", "/dbname")

	_, err := NewPgConfig()
	require.ErrorContains(t, err, "invalid database connection string")
}

func TestNewPgConfigReadsOptionalValues(t *testing.T) {
	t.Setenv("PG_MAX_POOL_SIZE", "12")
	t.Setenv("CLIENT_DB_CONFIG_FILE", "/tmp/client-db.json")
	t.Setenv("PG_IMPERSONATE_ROLE", "app_role")

	config, err := NewPgConfig()
	require.NoError(t, err)

	assert.Equal(t, 12, config.MaxPoolConnections)
	assert.Equal(t, "/tmp/client-db.json", config.ClientDbConfigFile)
	assert.Equal(t, "app_role", config.ImpersonateRole)
}
