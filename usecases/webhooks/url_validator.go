package webhooks

import (
	"context"
	"net"
	"net/url"
	"time"

	"github.com/checkmarble/marble-backend/infra"
	"github.com/checkmarble/marble-backend/models"
	"github.com/cockroachdb/errors"
)

// WebhookURLValidator validates webhook URLs for security concerns.
type WebhookURLValidator struct {
	allowInsecure bool         // Allow HTTP (only for development)
	ipWhitelist   []*net.IPNet // IP ranges to allow even if normally reserved
}

// NewWebhookURLValidator creates a validator.
// Set allowInsecure=true only for local development.
// ipWhitelist contains CIDR ranges that are allowed even if they would normally be blocked.
func NewWebhookURLValidator(allowInsecure bool, ipWhitelist []*net.IPNet) *WebhookURLValidator {
	return &WebhookURLValidator{
		allowInsecure: allowInsecure,
		ipWhitelist:   ipWhitelist,
	}
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
		if infra.IsReservedIP(ipAddr.IP, v.ipWhitelist) {
			return errors.Wrapf(models.BadParameterError,
				"URL resolves to a reserved IP address")
		}
	}

	return nil
}
