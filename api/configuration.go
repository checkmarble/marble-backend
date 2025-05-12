package api

import "time"

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
}
