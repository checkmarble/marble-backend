package usecases

import (
	"context"
	"net"
	"net/url"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/cockroachdb/errors"
)

// Reserved IP ranges that should never be webhook targets
var reservedIPBlocks []*net.IPNet

func init() {
	// Critical blocks that cover real-world SSRF attacks
	reserved := []string{
		"169.254.169.254/32", // Cloud metadata endpoint - CRITICAL
		"127.0.0.0/8",        // Localhost
		"10.0.0.0/8",         // Private (RFC1918)
		"172.16.0.0/12",      // Private (RFC1918)
		"192.168.0.0/16",     // Private (RFC1918)
		"::1/128",            // IPv6 localhost
		"fc00::/7",           // IPv6 unique local
		"fe80::/10",          // IPv6 link-local
	}

	for _, cidr := range reserved {
		_, block, _ := net.ParseCIDR(cidr)
		reservedIPBlocks = append(reservedIPBlocks, block)
	}
}

func isReservedIP(ip net.IP) bool {
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() {
		return true
	}
	for _, block := range reservedIPBlocks {
		if block.Contains(ip) {
			return true
		}
	}
	return false
}

// WebhookURLValidator validates webhook URLs for security concerns.
type WebhookURLValidator struct {
	allowInsecure bool // Allow HTTP (only for development)
}

// NewWebhookURLValidator creates a validator.
// Set allowInsecure=true only for local development.
func NewWebhookURLValidator(allowInsecure bool) *WebhookURLValidator {
	return &WebhookURLValidator{allowInsecure: allowInsecure}
}

// Validate checks that a URL is safe to use as a webhook endpoint.
// Returns an error if:
// - URL is malformed
// - Scheme is not HTTPS (or HTTP when allowInsecure=true)
// - URL contains credentials (user:pass@)
// - Hostname resolves to a reserved/private IP
func (v *WebhookURLValidator) Validate(ctx context.Context, rawURL string) error {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return errors.Wrap(models.BadParameterError, "invalid URL format")
	}

	// Check scheme
	if v.allowInsecure {
		if parsed.Scheme != "https" && parsed.Scheme != "http" {
			return errors.Wrap(models.BadParameterError, "URL scheme must be http or https")
		}
	} else {
		if parsed.Scheme != "https" {
			return errors.Wrap(models.BadParameterError, "URL scheme must be https")
		}
	}

	// Block credentials in URL (could leak in logs)
	if parsed.User != nil {
		return errors.Wrap(models.BadParameterError, "URL must not contain credentials")
	}

	// Extract hostname
	hostname := parsed.Hostname()
	if hostname == "" {
		return errors.Wrap(models.BadParameterError, "URL must have a hostname")
	}

	// Resolve hostname and check for reserved IPs
	resolver := &net.Resolver{}
	resolveCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	ips, err := resolver.LookupIPAddr(resolveCtx, hostname)
	if err != nil {
		return errors.Wrap(models.BadParameterError, "could not resolve hostname")
	}

	// Check ALL resolved IPs
	for _, ipAddr := range ips {
		if isReservedIP(ipAddr.IP) {
			return errors.Wrapf(models.BadParameterError,
				"URL resolves to a reserved IP address")
		}
	}

	return nil
}
