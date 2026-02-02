package worker_jobs

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestCalculateBackoff(t *testing.T) {
	t.Run("returns 0 for attempt 0 or negative", func(t *testing.T) {
		assert.Equal(t, time.Duration(0), CalculateBackoff(0))
		assert.Equal(t, time.Duration(0), CalculateBackoff(-1))
	})

	t.Run("returns correct delays for first attempts", func(t *testing.T) {
		// Attempt 1 uses delay[0] = 30s
		assert.Equal(t, 30*time.Second, CalculateBackoff(1))
		// Attempt 2 uses delay[1] = 2m
		assert.Equal(t, 2*time.Minute, CalculateBackoff(2))
		// Attempt 3 uses delay[2] = 10m
		assert.Equal(t, 10*time.Minute, CalculateBackoff(3))
		// Attempt 4 uses delay[3] = 1h
		assert.Equal(t, 1*time.Hour, CalculateBackoff(4))
		// Attempt 5 uses delay[4] = 4h
		assert.Equal(t, 4*time.Hour, CalculateBackoff(5))
		// Attempt 6 uses delay[5] = 12h
		assert.Equal(t, 12*time.Hour, CalculateBackoff(6))
	})

	t.Run("caps at max delay for high attempts", func(t *testing.T) {
		// Beyond defined delays, should use last delay (12h)
		assert.Equal(t, 12*time.Hour, CalculateBackoff(7))
		assert.Equal(t, 12*time.Hour, CalculateBackoff(10))
		assert.Equal(t, 12*time.Hour, CalculateBackoff(100))
	})
}

func TestWebhookSendResult_IsSuccess(t *testing.T) {
	t.Run("returns true for 2xx status codes", func(t *testing.T) {
		assert.True(t, WebhookSendResult{StatusCode: 200}.IsSuccess())
		assert.True(t, WebhookSendResult{StatusCode: 201}.IsSuccess())
		assert.True(t, WebhookSendResult{StatusCode: 204}.IsSuccess())
		assert.True(t, WebhookSendResult{StatusCode: 299}.IsSuccess())
	})

	t.Run("returns false for non-2xx status codes", func(t *testing.T) {
		assert.False(t, WebhookSendResult{StatusCode: 0}.IsSuccess())
		assert.False(t, WebhookSendResult{StatusCode: 199}.IsSuccess())
		assert.False(t, WebhookSendResult{StatusCode: 300}.IsSuccess())
		assert.False(t, WebhookSendResult{StatusCode: 400}.IsSuccess())
		assert.False(t, WebhookSendResult{StatusCode: 404}.IsSuccess())
		assert.False(t, WebhookSendResult{StatusCode: 500}.IsSuccess())
		assert.False(t, WebhookSendResult{StatusCode: 503}.IsSuccess())
	})
}
