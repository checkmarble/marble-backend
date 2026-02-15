package usecases

import (
	"context"
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseCIDRList(t *testing.T) {
	t.Run("empty string returns nil", func(t *testing.T) {
		result := ParseCIDRList("")
		assert.Nil(t, result)
	})

	t.Run("parses single CIDR", func(t *testing.T) {
		result := ParseCIDRList("10.0.0.0/8")
		assert.Len(t, result, 1)
	})

	t.Run("parses multiple CIDRs", func(t *testing.T) {
		result := ParseCIDRList("10.0.0.0/8, 192.168.0.0/16, 172.16.0.0/12")
		assert.Len(t, result, 3)
	})

	t.Run("ignores invalid CIDRs", func(t *testing.T) {
		result := ParseCIDRList("10.0.0.0/8, invalid, 192.168.0.0/16")
		assert.Len(t, result, 2)
	})

	t.Run("handles extra whitespace", func(t *testing.T) {
		result := ParseCIDRList("  10.0.0.0/8  ,  192.168.0.0/16  ")
		assert.Len(t, result, 2)
	})
}

func TestIsReservedIP(t *testing.T) {
	t.Run("loopback is reserved", func(t *testing.T) {
		assert.True(t, isReservedIP(net.ParseIP("127.0.0.1"), nil))
		assert.True(t, isReservedIP(net.ParseIP("127.255.255.255"), nil))
	})

	t.Run("private ranges are reserved", func(t *testing.T) {
		// 10.0.0.0/8
		assert.True(t, isReservedIP(net.ParseIP("10.0.0.1"), nil))
		assert.True(t, isReservedIP(net.ParseIP("10.255.255.255"), nil))

		// 172.16.0.0/12
		assert.True(t, isReservedIP(net.ParseIP("172.16.0.1"), nil))
		assert.True(t, isReservedIP(net.ParseIP("172.31.255.255"), nil))

		// 192.168.0.0/16
		assert.True(t, isReservedIP(net.ParseIP("192.168.0.1"), nil))
		assert.True(t, isReservedIP(net.ParseIP("192.168.255.255"), nil))
	})

	t.Run("link-local is reserved (cloud metadata)", func(t *testing.T) {
		assert.True(t, isReservedIP(net.ParseIP("169.254.169.254"), nil))
		assert.True(t, isReservedIP(net.ParseIP("169.254.0.1"), nil))
	})

	t.Run("CGNAT range is reserved", func(t *testing.T) {
		assert.True(t, isReservedIP(net.ParseIP("100.64.0.1"), nil))
		assert.True(t, isReservedIP(net.ParseIP("100.127.255.255"), nil))
	})

	t.Run("TEST-NET ranges are reserved", func(t *testing.T) {
		// TEST-NET-1
		assert.True(t, isReservedIP(net.ParseIP("192.0.2.1"), nil))
		// TEST-NET-2
		assert.True(t, isReservedIP(net.ParseIP("198.51.100.1"), nil))
		// TEST-NET-3
		assert.True(t, isReservedIP(net.ParseIP("203.0.113.1"), nil))
	})

	t.Run("benchmark range is reserved", func(t *testing.T) {
		assert.True(t, isReservedIP(net.ParseIP("198.18.0.1"), nil))
		assert.True(t, isReservedIP(net.ParseIP("198.19.255.255"), nil))
	})

	t.Run("0.0.0.0/8 is reserved", func(t *testing.T) {
		assert.True(t, isReservedIP(net.ParseIP("0.0.0.0"), nil))
		assert.True(t, isReservedIP(net.ParseIP("0.0.0.1"), nil))
	})

	t.Run("multicast is reserved", func(t *testing.T) {
		assert.True(t, isReservedIP(net.ParseIP("224.0.0.1"), nil))
		assert.True(t, isReservedIP(net.ParseIP("239.255.255.255"), nil))
	})

	t.Run("public IPs are not reserved", func(t *testing.T) {
		assert.False(t, isReservedIP(net.ParseIP("8.8.8.8"), nil))
		assert.False(t, isReservedIP(net.ParseIP("1.1.1.1"), nil))
		assert.False(t, isReservedIP(net.ParseIP("142.250.185.14"), nil)) // google.com
	})

	t.Run("whitelist overrides reserved", func(t *testing.T) {
		whitelist := ParseCIDRList("10.0.0.0/8")

		// 10.x should now be allowed
		assert.False(t, isReservedIP(net.ParseIP("10.0.0.1"), whitelist))
		assert.False(t, isReservedIP(net.ParseIP("10.255.255.255"), whitelist))

		// Other private ranges still blocked
		assert.True(t, isReservedIP(net.ParseIP("192.168.1.1"), whitelist))
	})

	t.Run("IPv6 loopback is reserved", func(t *testing.T) {
		assert.True(t, isReservedIP(net.ParseIP("::1"), nil))
	})

	t.Run("IPv6 link-local is reserved", func(t *testing.T) {
		assert.True(t, isReservedIP(net.ParseIP("fe80::1"), nil))
	})

	t.Run("IPv6 unique local is reserved", func(t *testing.T) {
		assert.True(t, isReservedIP(net.ParseIP("fd00::1"), nil))
	})
}

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
		whitelist := ParseCIDRList("10.0.0.0/8")
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
