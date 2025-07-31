package utils

type ContextKey int

const (
	ContextKeyCredentials ContextKey = iota
	ContextKeyClientIp
	ContextKeyLogger
	ContextKeySegmentClient
	ContextKeyOpenTelemetryTracer
)
