package utils

import (
	"context"

	"github.com/gin-gonic/gin"
	"github.com/segmentio/analytics-go/v3"
)

func SegmentClientFromContext(ctx context.Context) (analytics.Client, bool) {
	client, found := ctx.Value(ContextKeySegmentClient).(analytics.Client)
	return client, found
}

func StoreSegmentClientInContext(ctx context.Context, client analytics.Client) context.Context {
	return context.WithValue(ctx, ContextKeySegmentClient, client)
}

func StoreUsageTrackingEnabledInContext(ctx context.Context, enabled func(context.Context) bool) context.Context {
	return context.WithValue(ctx, ContextKeyUsageTrackingEnabled, enabled)
}

func UsageTrackingEnabledFromContext(ctx context.Context) (func(context.Context) bool, bool) {
	enabled, found := ctx.Value(ContextKeyUsageTrackingEnabled).(func(context.Context) bool)
	return enabled, found
}

func StoreSegmentClientInContextMiddleware(client analytics.Client, enabled func(context.Context) bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		ctxWithSegment := StoreSegmentClientInContext(ctx, client)
		ctxWithSegment = StoreUsageTrackingEnabledInContext(ctxWithSegment, enabled)
		c.Request = c.Request.WithContext(ctxWithSegment)
		c.Next()
	}
}
