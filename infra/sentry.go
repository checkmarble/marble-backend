package infra

import "github.com/getsentry/sentry-go"

func SetupSentry(dsn, env string) {
	if err := sentry.Init(sentry.ClientOptions{
		Dsn:           dsn,
		EnableTracing: true,
		Environment:   env,
		TracesSampler: sentry.TracesSampler(func(ctx sentry.SamplingContext) float64 {
			if ctx.Span.Name == "GET /liveness" {
				return 0.0
			}
			if ctx.Span.Name == "POST /ingestion/:object_type" {
				return 0.05
			}
			if ctx.Span.Name == "POST /decisions" {
				return 0.05
			}
			if ctx.Span.Name == "GET /token" {
				return 0.05
			}
			return 0.1
		}),
		// Experimental - value to be adjusted in prod once volumes go up - relative to the trace sampling rate
		ProfilesSampleRate: 0.2,
	}); err != nil {
		panic(err)
	}
}
