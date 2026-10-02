package utils

import (
	"context"

	"github.com/gin-gonic/gin"
	"github.com/segmentio/analytics-go/v3"
)

type UsageTracking interface {
	Enabled(context.Context) bool
}

func SegmentClientFromContext(ctx context.Context) (analytics.Client, bool) {
	client, found := ctx.Value(ContextKeySegmentClient).(analytics.Client)
	return client, found
}

func StoreSegmentClientInContext(ctx context.Context, client analytics.Client) context.Context {
	return context.WithValue(ctx, ContextKeySegmentClient, client)
}

func StoreUsageTrackingInContext(ctx context.Context, client analytics.Client, usageTracking UsageTracking) context.Context {
	ctx = StoreSegmentClientInContext(ctx, client)
	return context.WithValue(ctx, ContextKeyUsageTracking, usageTracking)
}

func UsageTrackingFromContext(ctx context.Context) (UsageTracking, bool) {
	usageTracking, found := ctx.Value(ContextKeyUsageTracking).(UsageTracking)
	return usageTracking, found
}

func StoreSegmentClientInContextMiddleware(client analytics.Client, usageTracking UsageTracking) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := StoreUsageTrackingInContext(c.Request.Context(), client, usageTracking)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}
