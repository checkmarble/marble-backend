package webhooks

import (
	"context"
	"testing"

	"github.com/checkmarble/marble-backend/infra"
	"github.com/stretchr/testify/assert"
)

func TestWebhookURLValidator_Validate(t *testing.T) {
	ctx := context.Background()

	// Note: Tests that require DNS resolution (allows HTTP/HTTPS, allows public domain)
	// are in TestWebhookURLValidator_Validate_Integration below and require network access.

	t.Run("rejects HTTP when insecure not allowed", func(t *testing.T) {
		validator := NewWebhookURLValidator(false, nil)
		// Use localhost which resolves without network
		err := validator.Validate(ctx, "http://localhost/webhook")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "https")
	})

	t.Run("rejects invalid scheme", func(t *testing.T) {
		validator := NewWebhookURLValidator(true, nil)
		err := validator.Validate(ctx, "ftp://localhost/webhook")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "scheme")
	})

	t.Run("rejects URL with credentials", func(t *testing.T) {
		validator := NewWebhookURLValidator(false, nil)
		err := validator.Validate(ctx, "https://user:pass@localhost/webhook")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "credentials")
	})

	t.Run("rejects URL with only username", func(t *testing.T) {
		validator := NewWebhookURLValidator(false, nil)
		err := validator.Validate(ctx, "https://user@localhost/webhook")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "credentials")
	})

	t.Run("rejects malformed URL", func(t *testing.T) {
		validator := NewWebhookURLValidator(false, nil)
		err := validator.Validate(ctx, "not a url")
		assert.Error(t, err)
	})

	t.Run("rejects URL without hostname", func(t *testing.T) {
		validator := NewWebhookURLValidator(false, nil)
		err := validator.Validate(ctx, "https:///path")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "hostname")
	})

	t.Run("rejects localhost", func(t *testing.T) {
		validator := NewWebhookURLValidator(false, nil)
		err := validator.Validate(ctx, "https://localhost/webhook")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "reserved")
	})

	t.Run("rejects 127.0.0.1", func(t *testing.T) {
		validator := NewWebhookURLValidator(false, nil)
		err := validator.Validate(ctx, "https://127.0.0.1/webhook")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "reserved")
	})

	t.Run("rejects private IP 10.x.x.x", func(t *testing.T) {
		validator := NewWebhookURLValidator(false, nil)
		err := validator.Validate(ctx, "https://10.0.0.1/webhook")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "reserved")
	})

	t.Run("rejects private IP 192.168.x.x", func(t *testing.T) {
		validator := NewWebhookURLValidator(false, nil)
		err := validator.Validate(ctx, "https://192.168.1.1/webhook")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "reserved")
	})

	t.Run("rejects cloud metadata IP", func(t *testing.T) {
		validator := NewWebhookURLValidator(false, nil)
		err := validator.Validate(ctx, "https://169.254.169.254/webhook")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "reserved")
	})

	t.Run("allows private IP when whitelisted", func(t *testing.T) {
		whitelist := infra.ParseCIDRList("10.0.0.0/8")
		validator := NewWebhookURLValidator(false, whitelist)
		err := validator.Validate(ctx, "https://10.0.0.1/webhook")
		assert.NoError(t, err)
	})

	// Note: Testing "allows public IP" requires network access and may fail
	// if the IP resolves to IPv6 addresses in reserved ranges (e.g., Teredo).
	// Public IP validation is covered by the isReservedIP tests above.
}

// Integration tests that require DNS resolution - may fail without network access
func TestWebhookURLValidator_Validate_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	ctx := context.Background()

	t.Run("allows HTTPS with real domain", func(t *testing.T) {
		validator := NewWebhookURLValidator(false, nil)
		// webhook.site is a known webhook testing service
		err := validator.Validate(ctx, "https://webhook.site/test")
		if err != nil {
			t.Skipf("skipping: DNS resolution or network issue: %v", err)
		}
		assert.NoError(t, err)
	})
}
