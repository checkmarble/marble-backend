package api

type Configuration struct {
	Env                  string
	AppName              string
	Port                 string
	MarbleAppHost        string
	MarbleBackofficeHost string
	RequestLoggingLevel  string
	TokenLifetimeMinute  int
	SegmentWriteKey      string
}
