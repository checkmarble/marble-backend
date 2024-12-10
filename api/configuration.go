package api

import "time"

type Configuration struct {
	Env                  string
	AppName              string
	Port                 string
	MarbleAppHost        string
	MarbleBackofficeHost string
	RequestLoggingLevel  string
	TokenLifetimeMinute  int
	SegmentWriteKey      string
	BatchTimeout         time.Duration
	DecisionTimeout      time.Duration
	DefaultTimeout       time.Duration
}
