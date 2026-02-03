package usecases

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/cockroachdb/errors"
)

const (
	// Default timeout for webhook HTTP requests
	DefaultWebhookTimeout = 30 * time.Second

	// MaxWebhookTimeout is the maximum allowed timeout (cannot be exceeded by customer config)
	MaxWebhookTimeout = 30 * time.Second

	// Timeout for endpoint validation pings
	ValidationPingTimeout = 10 * time.Second

	// User agent for webhook requests
	WebhookUserAgent = "Marble/1.0"

	// Header names
	HeaderConvoySignature  = "X-Convoy-Signature"
	HeaderMarbleApiVersion = "X-Marble-Api-Version"
	HeaderWebhookEventId   = "X-Webhook-Event-Id"
	HeaderWebhookEventType = "X-Webhook-Event-Type"
	HeaderContentType      = "Content-Type"
)

// WebhookDeliveryService handles HTTP delivery of webhooks.
type WebhookDeliveryService struct {
	httpClient       *http.Client
	signatureService *WebhookSignatureService
	apiVersion       string
}

// NewWebhookDeliveryService creates a new webhook delivery service.
func NewWebhookDeliveryService(apiVersion string) *WebhookDeliveryService {
	return &WebhookDeliveryService{
		httpClient: &http.Client{
			Timeout: DefaultWebhookTimeout,
		},
		signatureService: &WebhookSignatureService{},
		apiVersion:       apiVersion,
	}
}

// WebhookSendResult contains the result of a webhook delivery attempt.
type WebhookSendResult struct {
	StatusCode int
	Error      error
}

// IsSuccess returns true if the status code indicates success (2xx).
func (r WebhookSendResult) IsSuccess() bool {
	return r.StatusCode >= 200 && r.StatusCode < 300
}

// Send delivers a webhook payload to the specified endpoint.
func (s *WebhookDeliveryService) Send(
	ctx context.Context,
	webhook models.NewWebhook,
	secrets []models.NewWebhookSecret,
	event models.WebhookEventV2,
) WebhookSendResult {
	logger := utils.LoggerFromContext(ctx)

	// Set timeout based on webhook configuration, capped at MaxWebhookTimeout
	timeout := DefaultWebhookTimeout
	if webhook.HttpTimeoutSeconds > 0 {
		configuredTimeout := time.Duration(webhook.HttpTimeoutSeconds) * time.Second
		if configuredTimeout <= MaxWebhookTimeout {
			timeout = configuredTimeout
		}
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, webhook.Url, bytes.NewReader(event.EventData))
	if err != nil {
		return WebhookSendResult{Error: errors.Wrap(err, "failed to create request")}
	}

	// Generate signature
	timestamp := time.Now().Unix()
	signature := s.signatureService.Sign(event.EventData, secrets, timestamp)

	// Set headers
	req.Header.Set(HeaderContentType, "application/json")
	req.Header.Set(HeaderConvoySignature, signature)
	req.Header.Set(HeaderMarbleApiVersion, s.apiVersion)
	req.Header.Set(HeaderWebhookEventId, event.Id.String())
	req.Header.Set(HeaderWebhookEventType, event.EventType)
	req.Header.Set("User-Agent", WebhookUserAgent)

	logger.DebugContext(ctx, "Sending webhook",
		"url", webhook.Url,
		"event_type", event.EventType,
		"event_id", event.Id)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return WebhookSendResult{Error: errors.Wrap(err, "request failed")}
	}
	defer resp.Body.Close()

	// Drain body to allow connection reuse
	_, _ = io.Copy(io.Discard, resp.Body)

	logger.DebugContext(ctx, "Webhook response",
		"url", webhook.Url,
		"status_code", resp.StatusCode)

	return WebhookSendResult{StatusCode: resp.StatusCode}
}

// ValidateEndpoint validates a webhook endpoint by sending test requests.
// Tries HEAD → GET → POST until one returns 2xx.
func (s *WebhookDeliveryService) ValidateEndpoint(ctx context.Context, url string) error {
	client := &http.Client{Timeout: ValidationPingTimeout}
	methods := []string{http.MethodHead, http.MethodGet, http.MethodPost}

	var lastErr error
	for _, method := range methods {
		err := s.ping(ctx, client, url, method)
		if err == nil {
			return nil
		}
		lastErr = err
	}

	return errors.Wrap(lastErr, "endpoint validation failed: no 2xx response")
}

func (s *WebhookDeliveryService) ping(ctx context.Context, client *http.Client, url string, method string) error {
	var body io.Reader
	if method == http.MethodPost {
		body = bytes.NewBufferString(`{"test": "ping"}`)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return err
	}

	if method == http.MethodPost {
		req.Header.Set(HeaderContentType, "application/json")
	}
	req.Header.Set("User-Agent", WebhookUserAgent)

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	_, _ = io.Copy(io.Discard, resp.Body)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("received status %d", resp.StatusCode)
	}

	return nil
}
