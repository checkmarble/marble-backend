package infra

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/genai"
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

func TestNewAIAgentConfigurationDefaults(t *testing.T) {
	config := NewAIAgentConfiguration("gcp-project")

	assert.Equal(t, AIAgentProviderTypeOpenAI, config.MainAgentProviderType)
	assert.Equal(t, "gemini-2.5-flash", config.MainAgentDefaultModel)
	assert.Equal(t, "gcp-project", config.MainAgentProject)
}

func TestNewAIAgentConfigurationReadsEnv(t *testing.T) {
	t.Setenv("AI_AGENT_MAIN_AGENT_PROVIDER_TYPE", "aistudio")
	t.Setenv("AI_AGENT_MAIN_AGENT_URL", "https://example.test")
	t.Setenv("AI_AGENT_MAIN_AGENT_KEY", "secret")
	t.Setenv("AI_AGENT_MAIN_AGENT_DEFAULT_MODEL", "gpt-test")
	t.Setenv("AI_AGENT_MAIN_AGENT_BACKEND", "vertex")
	t.Setenv("AI_AGENT_MAIN_AGENT_PROJECT", "explicit-project")
	t.Setenv("AI_AGENT_MAIN_AGENT_LOCATION", "europe-west1")
	t.Setenv("AI_AGENT_PERPLEXITY_API_KEY", "pplx-key")

	config := NewAIAgentConfiguration("fallback-project")

	assert.Equal(t, AIAgentProviderTypeAIStudio, config.MainAgentProviderType)
	assert.Equal(t, "https://example.test", config.MainAgentURL)
	assert.Equal(t, "secret", config.MainAgentKey)
	assert.Equal(t, "gpt-test", config.MainAgentDefaultModel)
	assert.Equal(t, genai.BackendVertexAI, config.MainAgentBackend)
	assert.Equal(t, "explicit-project", config.MainAgentProject)
	assert.Equal(t, "europe-west1", config.MainAgentLocation)
	assert.Equal(t, "pplx-key", config.PerplexityAPIKey)
}


func TestNewConvoyConfigurationDefaults(t *testing.T) {
	config := NewConvoyConfiguration()

	assert.Equal(t, 50, config.RateLimit)
	assert.Equal(t, "", config.APIKey)
	assert.Equal(t, "", config.APIUrl)
	assert.Equal(t, "", config.ProjectID)
}

func TestNewConvoyConfigurationReadsEnv(t *testing.T) {
	t.Setenv("CONVOY_API_KEY", "key")
	t.Setenv("CONVOY_API_URL", "https://convoy.test")
	t.Setenv("CONVOY_PROJECT_ID", "project-id")
	t.Setenv("CONVOY_RATE_LIMIT", "25")

	config := NewConvoyConfiguration()

	assert.Equal(t, "key", config.APIKey)
	assert.Equal(t, "https://convoy.test", config.APIUrl)
	assert.Equal(t, "project-id", config.ProjectID)
	assert.Equal(t, 25, config.RateLimit)
}

func TestNewLicenseConfigurationDefaults(t *testing.T) {
	config := NewLicenseConfiguration()

	assert.Equal(t, "", config.LicenseKey)
	assert.False(t, config.KillIfReadLicenseError)
}

func TestNewLicenseConfigurationReadsEnv(t *testing.T) {
	t.Setenv("LICENSE_KEY", "license-key")
	t.Setenv("KILL_IF_READ_LICENSE_ERROR", "true")

	config := NewLicenseConfiguration()

	assert.Equal(t, "license-key", config.LicenseKey)
	assert.True(t, config.KillIfReadLicenseError)
}
