package infra

import (
	"github.com/cockroachdb/errors"
	"github.com/getsentry/sentry-go"
)

func SetupSentry(dsn, env, apiVersion string) {
	if err := sentry.Init(sentry.ClientOptions{
		Dsn:           dsn,
		EnableTracing: true,
		Release:       apiVersion,
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
			if ctx.Span.Name == "POST /transfers" {
				return 0.01
			}
			if ctx.Span.Name == "async_decision" {
				return 0.01
			}
			return 0.2
		}),
		// Experimental - value to be adjusted in prod once volumes go up - relative to the trace sampling rate
		ProfilesSampleRate: 0.2,
		BeforeSend: func(event *sentry.Event, hint *sentry.EventHint) *sentry.Event {
			if event.Request != nil {
				event.Request.Headers["X-Api-Key"] = "[redacted]"
			}
			if hint != nil && event != nil && len(event.Exception) > 0 {
				originalErr := errors.UnwrapAll(hint.OriginalException)
				event.Exception[len(event.Exception)-1].Type = originalErr.Error()
			}
			return event
		},
	}); err != nil {
		panic(err)
	}
}
