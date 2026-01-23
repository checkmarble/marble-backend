package infra

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/cockroachdb/errors"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/compute/v1"
	"google.golang.org/api/oauth2/v1"
	"google.golang.org/genai"
)

const (
	GcpServiceAccountSuffix = ".iam.gserviceaccount.com"
)

type GcpConfig struct {
	ProjectId      string
	PrincipalEmail string
}

func NewGcpConfig(ctx context.Context, gcpProjectId string, useFirebase bool) (GcpConfig, bool) {
	// Errors to validate GCP credentials do not have to be a hard fail.
	// They are common when trying out the product with the emulator (no service account required).
	// So long as the GCP project is defined in the configuration, most things will work.
	// Failing to retrieve the principal is OK for now, but will become an error when we implement keyless signing.

	logger := utils.LoggerFromContext(ctx)
	adcProjectId, adcPrincipal, err := FindServiceAccountPrincipal(ctx)
	if err != nil {
		switch useFirebase {
		case true:
			logger.ErrorContext(ctx, "Could not validate Google Cloud credentials, some features might not work properly", "error", err)
		case false:
			logger.DebugContext(ctx, "Could not validate Google Cloud credentials, some specific features may be disabled (GCP tracing, profiling, and GCS-backed file storage)")
		}
		return GcpConfig{}, false
	}

	if adcPrincipal != "" && !strings.HasSuffix(adcPrincipal, GcpServiceAccountSuffix) {
		logger.WarnContext(ctx, "You might be authenticated with user Google user account (instead of a service account), file downloads will not be functional")
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
		return GcpConfig{}, false
	}

	utils.LoggerFromContext(ctx).InfoContext(ctx, "Authenticated in GCP", "principal", adcPrincipal, "project", projectId)
	cfg := GcpConfig{
		ProjectId:      projectId,
		PrincipalEmail: adcPrincipal,
	}
	return cfg, true
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

	// Role to impersonate when connecting to the database. To be used in particular with IAM authentication t
	// handle role based access control. Ignored if empty.
	ImpersonateRole string
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
	Exporter        string
	SamplingMap     TelemetrySamplingMap
}

type TelemetrySamplingMap struct {
	SpanNames  map[string]float64 `json:"span_names"`
	HttpRoutes map[string]float64 `json:"http_routes"`
}

func NewTelemetrySamplingMap(ctx context.Context, path string) TelemetrySamplingMap {
	m := TelemetrySamplingMap{
		SpanNames: make(map[string]float64),
	}

	if path == "" {
		return m
	}

	f, err := os.Open(path)
	if err != nil {
		utils.LoggerFromContext(ctx).Warn(fmt.Sprintf(
			"could not read otel sampling rates file: %s", err.Error()))
		return m
	}

	if err := json.NewDecoder(f).Decode(&m); err != nil {
		utils.LoggerFromContext(ctx).Warn(fmt.Sprintf(
			"could not read otel sampling rates file: %s", err.Error()))
		return m
	}

	return m
}

type AIAgentProviderType string

const (
	AIAgentProviderTypeOpenAI   AIAgentProviderType = "openai"
	AIAgentProviderTypeAIStudio AIAgentProviderType = "aistudio"
	AIAgentProviderTypeUnknown  AIAgentProviderType = "unknown"
)

type AIAgentConfiguration struct {
	MainAgentProviderType AIAgentProviderType
	MainAgentURL          string
	MainAgentKey          string
	MainAgentDefaultModel string

	// For AI Studio
	MainAgentBackend  genai.Backend
	MainAgentProject  string
	MainAgentLocation string

	// For Perplexity
	PerplexityAPIKey string
}

func AIAgentProviderTypeFromString(providerType string) AIAgentProviderType {
	switch providerType {
	case "openai":
		return AIAgentProviderTypeOpenAI
	case "aistudio":
		return AIAgentProviderTypeAIStudio
	default:
		return AIAgentProviderTypeUnknown
	}
}

func AIAgentProviderBackendFromString(providerBackend string) genai.Backend {
	switch providerBackend {
	case "gemini":
		return genai.BackendGeminiAPI
	case "vertex":
		return genai.BackendVertexAI
	default:
		return genai.BackendUnspecified
	}
}
