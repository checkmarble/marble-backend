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

type conditionalSegmentClient struct {
	analytics.Client
	ctx     context.Context
	enabled func(context.Context) bool
}

func (client conditionalSegmentClient) Enqueue(message analytics.Message) error {
	if !client.enabled(client.ctx) {
		return nil
	}
	return client.Client.Enqueue(message)
}

func StoreSegmentClientInContextMiddleware(client analytics.Client, enabled func(context.Context) bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		ctxWithSegment := StoreSegmentClientInContext(ctx, conditionalSegmentClient{
			Client:  client,
			ctx:     ctx,
			enabled: enabled,
		})
		c.Request = c.Request.WithContext(ctxWithSegment)
		c.Next()
	}
}
