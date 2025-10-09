package lago_repository

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/checkmarble/marble-backend/infra"
	"github.com/checkmarble/marble-backend/models"
	"github.com/h2non/gock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSendEvent(t *testing.T) {
	defer gock.Off()

	// Setup mock HTTP response - verifies headers and payload format
	gock.New("https://api.getlago.com").
		Post("/api/v1/events").
		MatchHeader("Authorization", "Bearer test_api_key").
		MatchHeader("Content-Type", "application/json").
		AddMatcher(func(req *http.Request, _ *gock.Request) (bool, error) {
			bodyBytes, _ := io.ReadAll(req.Body)

			// Verify payload format: { event: { transaction_id, code, external_subscription_id } }
			var body map[string]interface{}
			if err := json.Unmarshal(bodyBytes, &body); err != nil {
				t.Errorf("Body should be valid JSON: %v", err)
				return false, err
			}

			// Check first layer has "event"
			event, ok := body["event"].(map[string]interface{})
			if !ok {
				t.Errorf("Body should have 'event' field")
				return false, nil
			}

			// Check required fields in event
			assert.Equal(t, "txn_123", event["transaction_id"])
			assert.Equal(t, "sub_456", event["external_subscription_id"])
			assert.Equal(t, "api_calls", event["code"])

			return true, nil
		}).
		Reply(http.StatusOK)

	client := &http.Client{Transport: &http.Transport{}}
	gock.InterceptClient(client)

	repo := LagoRepository{
		client: client,
		lagoConfig: infra.LagoConfig{
			BaseUrl: "https://api.getlago.com",
			ApiKey:  "test_api_key",
		},
	}

	event := models.BillingEvent{
		TransactionId:          "txn_123",
		ExternalSubscriptionId: "sub_456",
		Code:                   "api_calls",
		Timestamp:              time.Now(),
		Properties:             map[string]any{"count": 10},
	}

	// Execute
	err := repo.SendEvent(context.Background(), event)

	// Assert
	assert.NoError(t, err)
	assert.True(t, gock.IsDone(), "All HTTP mocks should have been called")
}

func TestSendEvents(t *testing.T) {
	defer gock.Off()

	// Setup mock HTTP response - verifies headers and payload format
	gock.New("https://api.getlago.com").
		Post("/api/v1/events/batch").
		MatchHeader("Authorization", "Bearer test_api_key").
		MatchHeader("Content-Type", "application/json").
		AddMatcher(func(req *http.Request, _ *gock.Request) (bool, error) {
			bodyBytes, _ := io.ReadAll(req.Body)

			// Verify payload format: { events: [ { transaction_id, code, external_subscription_id }, ... ] }
			var body map[string]interface{}
			if err := json.Unmarshal(bodyBytes, &body); err != nil {
				t.Errorf("Body should be valid JSON: %v", err)
				return false, err
			}

			// Check first layer has "events"
			events, ok := body["events"].([]interface{})
			if !ok {
				t.Errorf("Body should have 'events' array")
				return false, nil
			}

			require.Len(t, events, 2)

			// Check first event has required fields
			firstEvent, ok := events[0].(map[string]interface{})
			if !ok {
				t.Errorf("Event should be an object")
				return false, nil
			}

			assert.Equal(t, "txn_1", firstEvent["transaction_id"])
			assert.Equal(t, "sub_123", firstEvent["external_subscription_id"])
			assert.Equal(t, "api_calls", firstEvent["code"])

			return true, nil
		}).
		Reply(http.StatusOK)

	client := &http.Client{Transport: &http.Transport{}}
	gock.InterceptClient(client)

	repo := LagoRepository{
		client: client,
		lagoConfig: infra.LagoConfig{
			BaseUrl: "https://api.getlago.com",
			ApiKey:  "test_api_key",
		},
	}

	events := []models.BillingEvent{
		{
			TransactionId:          "txn_1",
			ExternalSubscriptionId: "sub_123",
			Code:                   "api_calls",
			Timestamp:              time.Now(),
			Properties:             map[string]any{},
		},
		{
			TransactionId:          "txn_2",
			ExternalSubscriptionId: "sub_123",
			Code:                   "storage",
			Timestamp:              time.Now(),
			Properties:             map[string]any{},
		},
	}

	// Execute
	err := repo.SendEvents(context.Background(), events)

	// Assert
	assert.NoError(t, err)
	assert.True(t, gock.IsDone(), "All HTTP mocks should have been called")
}

func TestSendEvents_LargeBatch(t *testing.T) {
	defer gock.Off()

	// Track number of batches received
	batchCount := 0

	// Setup mock HTTP response - should be called twice for 150 events
	gock.New("https://api.getlago.com").
		Post("/api/v1/events/batch").
		MatchHeader("Authorization", "Bearer test_api_key").
		MatchHeader("Content-Type", "application/json").
		AddMatcher(func(req *http.Request, _ *gock.Request) (bool, error) {
			bodyBytes, _ := io.ReadAll(req.Body)

			// Verify payload format
			var body map[string]interface{}
			if err := json.Unmarshal(bodyBytes, &body); err != nil {
				t.Errorf("Body should be valid JSON: %v", err)
				return false, err
			}

			// Check first layer has "events"
			events, ok := body["events"].([]interface{})
			if !ok {
				t.Errorf("Body should have 'events' array")
				return false, nil
			}

			batchCount++

			// First batch should have 100 events, second batch should have 50
			if batchCount == 1 {
				assert.Len(t, events, 100, "First batch should have 100 events")
			} else if batchCount == 2 {
				assert.Len(t, events, 50, "Second batch should have 50 events")
			}

			// Check first event has required fields
			if len(events) > 0 {
				firstEvent, ok := events[0].(map[string]interface{})
				if !ok {
					t.Errorf("Event should be an object")
					return false, nil
				}

				assert.NotEmpty(t, firstEvent["transaction_id"])
				assert.NotEmpty(t, firstEvent["external_subscription_id"])
				assert.NotEmpty(t, firstEvent["code"])
			}

			return true, nil
		}).
		Times(2). // Expect 2 calls for 150 events
		Reply(http.StatusOK)

	client := &http.Client{Transport: &http.Transport{}}
	gock.InterceptClient(client)

	repo := LagoRepository{
		client: client,
		lagoConfig: infra.LagoConfig{
			BaseUrl: "https://api.getlago.com",
			ApiKey:  "test_api_key",
		},
	}

	// Create 150 events (more than MAX_EVENTS_PER_BATCH of 100)
	events := make([]models.BillingEvent, 150)
	for i := 0; i < 150; i++ {
		events[i] = models.BillingEvent{
			TransactionId:          "txn_" + string(rune(i)),
			ExternalSubscriptionId: "sub_123",
			Code:                   "api_calls",
			Timestamp:              time.Now(),
			Properties:             map[string]any{},
		}
	}

	// Execute
	err := repo.SendEvents(context.Background(), events)

	// Assert
	assert.NoError(t, err)
	assert.Equal(t, 2, batchCount, "Should have sent 2 batches")
	assert.True(t, gock.IsDone(), "All HTTP mocks should have been called")
}
