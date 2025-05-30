package infra

import (
	"context"
	"fmt"
	"strings"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/cockroachdb/errors"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/compute/v1"
	"google.golang.org/api/oauth2/v1"
)

const (
	GcpServiceAccountSuffix = ".iam.gserviceaccount.com"
)

type GcpConfig struct {
	ProjectId                    string
	PrincipalEmail               string
	GoogleApplicationCredentials string
	EnableTracing                bool
}

func NewGcpConfig(ctx context.Context, gcpProjectId string, googleApplicationCredentials string, enableTracing bool) (GcpConfig, error) {
	// Errors to validate GCP credentials do not have to be a hard fail.
	// They are common when trying out the product with the emulator (no service account required).
	// So long as the GCP project is defined in the configuration, most things will work.
	// Failing to retrieve the principal is OK for now, but will become an error when we implement keyless signing.
	adcProjectId, adcPrincipal, err := FindServiceAccountPrincipal(ctx)
	if err != nil {
		utils.LoggerFromContext(ctx).Warn("could not validate Google Cloud credentials, some features might not work properly", "error", err)
	}

	if !strings.HasSuffix(adcPrincipal, GcpServiceAccountSuffix) {
		utils.LoggerFromContext(ctx).Warn("you might be authenticated with user Google user account (instead of a service account), file downloads will not be functional")
	}

	// We determine the Google Cloud project in the following priority:
	//  - The value of GOOGLE_CLOUD_PROJECT, that can override automatic detection
	//  - The project we detect from the ADC / private key file
	//  - The project extracted from the service account email address
	var projectId string

	if projectId == "" && gcpProjectId != "" {
		projectId = gcpProjectId
	}
	if projectId == "" && adcProjectId != "" {
		projectId = adcProjectId
	}
	// In the case ADC is authenticated with a user account, and even when impersonating a
	// service account, the project will not be detected here, so we extract it from the
	// service account email.
	if projectId == "" && strings.HasSuffix(adcPrincipal, GcpServiceAccountSuffix) {
		if _, domain, ok := strings.Cut(adcPrincipal, "@"); ok {
			projectId = strings.TrimSuffix(domain, GcpServiceAccountSuffix)
		}
	}

	if projectId == "" {
		return GcpConfig{}, errors.New("could not detect Google Cloud project ID, you must set GOOGLE_CLOUD_PROJECT")
	}

	cfg := GcpConfig{
		ProjectId:                    projectId,
		PrincipalEmail:               adcPrincipal,
		GoogleApplicationCredentials: googleApplicationCredentials,
		EnableTracing:                enableTracing,
	}

	return cfg, nil
}

func FindServiceAccountPrincipal(ctx context.Context) (string, string, error) {
	creds, err := google.FindDefaultCredentials(ctx, compute.CloudPlatformScope)
	if err != nil {
		return "", "", errors.Wrap(err, "credentials not found (set GOOGLE_APPLICATION_CREDENTIALS)")
	}

	svc, err := oauth2.NewService(ctx)
	if err != nil {
		return "", "", errors.Wrap(err, "could not create token service")
	}
	tokenInfo, err := svc.Tokeninfo().Do()
	if err != nil {
		return "", "", errors.Wrap(err, "could not obtain token from application credentials")
	}

	return creds.ProjectID, tokenInfo.Email, nil
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
