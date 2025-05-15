package api

import (
	"time"

	"github.com/checkmarble/marble-backend/infra"
)

type Configuration struct {
	Env                 string
	AppName             string
	Port                string
	MarbleAppUrl        string
	MarbleBackofficeUrl string
	RequestLoggingLevel string
	TokenLifetimeMinute int
	SegmentWriteKey     string
	DisableSegment      bool
	BatchTimeout        time.Duration
	DecisionTimeout     time.Duration
	DefaultTimeout      time.Duration

	FirebaseConfig FirebaseConfig
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
