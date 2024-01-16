package utils

type ContextKey int

const (
	ContextKeyCredentials ContextKey = iota
	ContextKeyLogger
	ContextKeySegmentClient
	ContextKeyOpenTelemetryTracer
)
