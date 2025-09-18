package api

import (
	"time"

	"github.com/checkmarble/marble-backend/infra"
	"github.com/checkmarble/marble-backend/usecases/auth"
)

type Configuration struct {
	Env                 string
	AppName             string
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

	AnalyticsEnabled bool
	AnalyticsTimeout time.Duration

	TokenProvider  auth.TokenProvider
	FirebaseConfig FirebaseConfig
	OidcConfig     infra.OidcConfig

	MetabaseConfig infra.MetabaseConfiguration
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
