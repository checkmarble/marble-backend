package usecases

import (
	"testing"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestWebhookSignatureService_Sign(t *testing.T) {
	service := &WebhookSignatureService{}

	t.Run("generates signature with single secret", func(t *testing.T) {
		payload := []byte(`{"event": "test"}`)
		secrets := []models.NewWebhookSecret{
			{
				Id:        uuid.New(),
				WebhookId: uuid.New(),
				Value:     "test-secret-key",
				CreatedAt: time.Now(),
			},
		}
		timestamp := int64(1706745600) // Fixed timestamp for deterministic test

		signature := service.Sign(payload, secrets, timestamp)

		assert.NotEmpty(t, signature)
		assert.Contains(t, signature, "t=1706745600")
		assert.Contains(t, signature, "v1=")
	})

	t.Run("generates multiple signatures for secret rotation", func(t *testing.T) {
		payload := []byte(`{"event": "test"}`)
		secrets := []models.NewWebhookSecret{
			{
				Id:        uuid.New(),
				WebhookId: uuid.New(),
				Value:     "secret-key-1",
				CreatedAt: time.Now(),
			},
			{
				Id:        uuid.New(),
				WebhookId: uuid.New(),
				Value:     "secret-key-2",
				CreatedAt: time.Now().Add(-time.Hour),
			},
		}
		timestamp := int64(1706745600)

		signature := service.Sign(payload, secrets, timestamp)

		assert.NotEmpty(t, signature)
		// Should have v1= and v2= entries (incrementing version numbers)
		assert.Contains(t, signature, "t=1706745600,v1=")
		assert.Contains(t, signature, ",v2=")
	})

	t.Run("returns empty string for no secrets", func(t *testing.T) {
		payload := []byte(`{"event": "test"}`)
		secrets := []models.NewWebhookSecret{}
		timestamp := int64(1706745600)

		signature := service.Sign(payload, secrets, timestamp)

		assert.Empty(t, signature)
	})

	t.Run("produces consistent signatures", func(t *testing.T) {
		payload := []byte(`{"event": "test"}`)
		secrets := []models.NewWebhookSecret{
			{
				Id:        uuid.New(),
				WebhookId: uuid.New(),
				Value:     "consistent-secret",
				CreatedAt: time.Now(),
			},
		}
		timestamp := int64(1706745600)

		sig1 := service.Sign(payload, secrets, timestamp)
		sig2 := service.Sign(payload, secrets, timestamp)

		assert.Equal(t, sig1, sig2)
	})

	t.Run("different payloads produce different signatures", func(t *testing.T) {
		secrets := []models.NewWebhookSecret{
			{
				Id:        uuid.New(),
				WebhookId: uuid.New(),
				Value:     "test-secret",
				CreatedAt: time.Now(),
			},
		}
		timestamp := int64(1706745600)

		sig1 := service.Sign([]byte(`{"event": "test1"}`), secrets, timestamp)
		sig2 := service.Sign([]byte(`{"event": "test2"}`), secrets, timestamp)

		assert.NotEqual(t, sig1, sig2)
	})

	t.Run("different timestamps produce different signatures", func(t *testing.T) {
		payload := []byte(`{"event": "test"}`)
		secrets := []models.NewWebhookSecret{
			{
				Id:        uuid.New(),
				WebhookId: uuid.New(),
				Value:     "test-secret",
				CreatedAt: time.Now(),
			},
		}

		sig1 := service.Sign(payload, secrets, int64(1706745600))
		sig2 := service.Sign(payload, secrets, int64(1706745601))

		assert.NotEqual(t, sig1, sig2)
	})
}

func TestGenerateWebhookSecret(t *testing.T) {
	t.Run("generates 64 character hex string", func(t *testing.T) {
		secret, err := GenerateWebhookSecret()

		assert.NoError(t, err)
		assert.Len(t, secret, 64) // 32 bytes = 64 hex chars
	})

	t.Run("generates unique secrets", func(t *testing.T) {
		secrets := make(map[string]bool)
		for i := 0; i < 100; i++ {
			secret, err := GenerateWebhookSecret()
			assert.NoError(t, err)
			assert.False(t, secrets[secret], "should not generate duplicate secrets")
			secrets[secret] = true
		}
	})
}
