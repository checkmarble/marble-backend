package api

import (
	"time"

	"github.com/checkmarble/marble-backend/infra"
	"github.com/checkmarble/marble-backend/usecases/auth"
)

type ServerMode int

const (
	ServerModeDefault ServerMode = iota
	ServerModeAnalytics
)

type Configuration struct {
	Env                 string
	AppName             string
	AppVersion          string
	ServerMode          ServerMode
	Port                string
	MarbleApiUrl        string
	MarbleAppUrl        string
	MarbleBackofficeUrl string
	RequestLoggingLevel string
	TokenLifetimeMinute int
	SegmentWriteKey     string
	DisableSegment      bool
	BatchTimeout        time.Duration
	DecisionTimeout     time.Duration
	DefaultTimeout      time.Duration

	AnalyticsEnabled     bool
	AnalyticsTimeout     time.Duration
	AnalyticsProxyApiUrl string

	TokenProvider  auth.TokenProvider
	FirebaseConfig FirebaseConfig
	OidcConfig     infra.OidcConfig

	GcpConfig        infra.GcpConfig
	MetabaseConfig   infra.MetabaseConfiguration
	EnablePrometheus bool
}

type FirebaseConfig struct {
	EmulatorHost string
	ProjectId    string
	ApiKey       string
	AuthDomain   string
}

func (cfg FirebaseConfig) IsEmulator() bool {
	return cfg.EmulatorHost != ""
}
