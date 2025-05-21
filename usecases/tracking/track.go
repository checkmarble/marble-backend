package tracking

import (
	"context"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"

	"github.com/segmentio/analytics-go/v3"
)

func TrackEvent(ctx context.Context, event models.AnalyticsEvent, properties map[string]interface{}) {
	credentials, segmentClient, found := getCredentialsAndAnalyticsClientFromContext(ctx)
	if !found {
		return
	}

	segmentProperties := analytics.NewProperties()
	for k, v := range properties {
		segmentProperties.Set(k, v)
	}
	err := segmentClient.Enqueue(analytics.Track{
		Event:      string(event),
		UserId:     string(credentials.ActorIdentity.UserId),
		Properties: segmentProperties,
	})
	if err != nil {
		logger := utils.LoggerFromContext(ctx)
		logger.ErrorContext(ctx, "Failed to track event", "error", err.Error())
	}
}

func TrackEventWithUserId(ctx context.Context, event models.AnalyticsEvent, userId models.UserId, properties map[string]interface{}) {
	segmentClient, found := getAnalyticsClientFromContext(ctx)
	if !found {
		return
	}

	segmentProperties := analytics.NewProperties()
	for k, v := range properties {
		segmentProperties.Set(k, v)
	}

	err := segmentClient.Enqueue(analytics.Track{
		Event:      string(event),
		UserId:     string(userId),
		Properties: segmentProperties,
	})
	if err != nil {
		logger := utils.LoggerFromContext(ctx)
		logger.ErrorContext(ctx, "Failed to track event", "error", err.Error())
	}
}

func Identify(ctx context.Context, userId models.UserId, traits map[string]interface{}) {
	segmentClient, found := utils.SegmentClientFromContext(ctx)
	if !found || segmentClient == nil {
		return
	}

	segmentTraits := analytics.NewTraits()
	for k, v := range traits {
		segmentTraits.Set(k, v)
	}

	err := segmentClient.Enqueue(analytics.Identify{
		UserId: string(userId),
		Traits: segmentTraits,
	})
	if err != nil {
		logger := utils.LoggerFromContext(ctx)
		logger.ErrorContext(ctx, "Failed to track event", "error", err.Error())
	}
}

func Group(ctx context.Context, userId models.UserId, organizationId string, traits map[string]interface{}) {
	segmentClient, found := utils.SegmentClientFromContext(ctx)
	if !found || segmentClient == nil {
		return
	}

	segmentTraits := analytics.NewTraits()
	for k, v := range traits {
		segmentTraits.Set(k, v)
	}

	err := segmentClient.Enqueue(analytics.Group{
		UserId:  string(userId),
		GroupId: organizationId,
		Traits:  segmentTraits,
	})
	if err != nil {
		logger := utils.LoggerFromContext(ctx)
		logger.ErrorContext(ctx, "Failed to track event", "error", err.Error())
	}
}

func getCredentialsAndAnalyticsClientFromContext(ctx context.Context) (models.Credentials, analytics.Client, bool) {
	logger := utils.LoggerFromContext(ctx)
	credentials, found := utils.CredentialsFromCtx(ctx)
	if !found {
		logger.ErrorContext(ctx, "Credentials not found in context")
		return models.Credentials{}, nil, false
	}
	segmentClient, found := getAnalyticsClientFromContext(ctx)
	if !found {
		return credentials, nil, false
	}
	return credentials, segmentClient, true
}

func getAnalyticsClientFromContext(ctx context.Context) (analytics.Client, bool) {
	segmentClient, found := utils.SegmentClientFromContext(ctx)
	if !found || segmentClient == nil {
		return nil, false
	}
	return segmentClient, true
}
