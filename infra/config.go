package infra

import (
	"fmt"

	"github.com/checkmarble/marble-backend/models"
)

type GcpConfig struct {
	ProjectId                    string
	GoogleApplicationCredentials string
	EnableTracing                bool
}

type PgConfig struct {
	ConnectionString   string
	Database           string
	Hostname           string
	Password           string
	Port               string
	User               string
	MaxPoolConnections int
	ClientDbConfigFile string
	SslMode            string
}

func (config PgConfig) GetConnectionString() string {
	if config.ConnectionString != "" {
		return config.ConnectionString
	}

	if config.Hostname == "" || config.User == "" || config.Password == "" || config.Database == "" {
		panic("Missing required configuration for connecting to PostgreSQL in PgConfig")
	}

	if config.SslMode == "" {
		config.SslMode = "prefer"
	}

	connectionString := fmt.Sprintf("host=%s user=%s password=%s database=%s sslmode=%s",
		config.Hostname, config.User, config.Password, config.Database, config.SslMode)
	if config.Port != "" {
		// In some cases, the port is not required. E.g. Cloud Run connects to the DB through a managed proxy
		// and a unix socket, so we don't need need to specify the port in that case.
		connectionString = fmt.Sprintf("%s port=%s", connectionString, config.Port)
	}
	return connectionString
}

type MetabaseConfiguration struct {
	SiteUrl             string
	JwtSigningKey       []byte
	TokenLifetimeMinute int
	Resources           map[models.EmbeddingType]int
}

type TelemetryConfiguration struct {
	Enabled         bool
	ApplicationName string
	ProjectID       string
}

type ConvoyConfiguration struct {
	APIKey    string
	APIUrl    string
	ProjectID string
	RateLimit int
}
