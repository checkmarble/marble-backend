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
	ConnectionString    string
	Database            string
	DbConnectWithSocket bool
	Hostname            string
	Password            string
	Port                string
	User                string
	MaxPoolConnections  int
	ClientDbConfigFile  string
	SslMode             string
}

func (config PgConfig) GetConnectionString() string {
	if config.ConnectionString != "" {
		return config.ConnectionString
	}

	if config.SslMode == "" {
		config.SslMode = "prefer"
	}

	connectionString := fmt.Sprintf("host=%s user=%s password=%s database=%s sslmode=%s",
		config.Hostname, config.User, config.Password, config.Database, config.SslMode)
	if !config.DbConnectWithSocket {
		// Cloud Run connects to the DB through a proxy and a unix socket, so we don't need need to specify the port
		// but we do when running locally
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
