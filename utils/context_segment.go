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

func StoreSegmentClientInContextMiddleware(client analytics.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctxWithSegment := StoreSegmentClientInContext(c.Request.Context(), client)
		c.Request = c.Request.WithContext(ctxWithSegment)
		c.Next()
	}
}
