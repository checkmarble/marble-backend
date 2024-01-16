package utils

import (
	"context"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
)

func OpenTelemetryTracerFromContext(ctx context.Context) trace.Tracer {
	tracer, found := ctx.Value(ContextKeyOpenTelemetryTracer).(trace.Tracer)

	if !found {
		LoggerFromContext(ctx).DebugContext(ctx, "OpenTelemetryTracer not found in context, using NoopTracer: traces will be dismissed")
		return &noop.Tracer{}
	}

	return tracer
}

func StoreOpenTelemetryTracerInContext(ctx context.Context, tracer trace.Tracer) context.Context {
	return context.WithValue(ctx, ContextKeyOpenTelemetryTracer, tracer)
}

func StoreOpenTelemetryTracerInContextMiddleware(tracer trace.Tracer) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctxWithTracer := StoreOpenTelemetryTracerInContext(c.Request.Context(), tracer)
		c.Request = c.Request.WithContext(ctxWithTracer)
		c.Next()
	}
}
