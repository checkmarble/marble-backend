package usecases

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
)

// Exponential backoff retry schedule
var retrySchedule = []time.Duration{
	1 * time.Minute,
	5 * time.Minute,
	30 * time.Minute,
	2 * time.Hour,
	8 * time.Hour,
	24 * time.Hour,
}

const maxRetryAttempts = 24

type webhookDeliveryRepository interface {
	GetWebhook(ctx context.Context, exec repositories.Executor, id uuid.UUID) (models.Webhook, error)
	ListActiveSecrets(ctx context.Context, exec repositories.Executor, webhookId uuid.UUID) ([]models.Secret, error)
	GetWebhookDelivery(ctx context.Context, exec repositories.Executor, id uuid.UUID) (models.WebhookDelivery, error)
	MarkWebhookDeliverySuccess(ctx context.Context, exec repositories.Executor, id uuid.UUID, responseStatus int) error
	MarkWebhookDeliveryFailed(ctx context.Context, exec repositories.Executor, id uuid.UUID, errMsg string, responseStatus *int, nextRetryAt *time.Time, attempts int) error
}

type WebhookDeliveryService struct {
	httpClient      *http.Client
	repository      webhookDeliveryRepository
	executorFactory executor_factory.ExecutorFactory
}

func NewWebhookDeliveryService(
	repository webhookDeliveryRepository,
	executorFactory executor_factory.ExecutorFactory,
) *WebhookDeliveryService {
	return &WebhookDeliveryService{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		repository:      repository,
		executorFactory: executorFactory,
	}
}

// generateSignature creates an HMAC-SHA256 signature
func (s *WebhookDeliveryService) generateSignature(secret string, payload []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	return hex.EncodeToString(mac.Sum(nil))
}

// buildSignatureHeader builds the signature header
// - Single secret: just the hex-encoded signature
// - Multiple secrets (rotation): "t=<timestamp>,v1=<sig1>,v1=<sig2>"
func (s *WebhookDeliveryService) buildSignatureHeader(secrets []models.Secret, timestamp int64, payload []byte) string {
	if len(secrets) == 0 {
		return ""
	}

	if len(secrets) == 1 {
		// Simple format for single secret
		return s.generateSignature(secrets[0].Value, payload)
	}

	// Multi-signature format during rotation
	var sigs []string
	for _, secret := range secrets {
		sig := s.generateSignature(secret.Value, payload)
		sigs = append(sigs, fmt.Sprintf("v1=%s", sig))
	}
	return fmt.Sprintf("t=%d,%s", timestamp, strings.Join(sigs, ","))
}

// getNextRetryTime calculates the next retry time based on the number of attempts
func (s *WebhookDeliveryService) getNextRetryTime(attempts int) *time.Time {
	if attempts >= maxRetryAttempts {
		return nil // Stop retrying
	}

	var delay time.Duration
	if attempts < len(retrySchedule) {
		delay = retrySchedule[attempts]
	} else {
		// After schedule exhausted, retry every 24 hours
		delay = 24 * time.Hour
	}

	t := time.Now().Add(delay)
	return &t
}

// DeliverWebhook delivers a webhook event to an endpoint
func (s *WebhookDeliveryService) DeliverWebhook(
	ctx context.Context,
	delivery models.WebhookDelivery,
	event models.WebhookEvent,
	payload json.RawMessage,
) error {
	logger := utils.LoggerFromContext(ctx)
	exec := s.executorFactory.NewExecutor()

	// Get webhook configuration
	webhook, err := s.repository.GetWebhook(ctx, exec, delivery.WebhookId)
	if err != nil {
		errMsg := fmt.Sprintf("failed to get webhook: %s", err.Error())
		logger.ErrorContext(ctx, errMsg, "delivery_id", delivery.Id, "webhook_id", delivery.WebhookId)
		return s.repository.MarkWebhookDeliveryFailed(ctx, exec, delivery.Id, errMsg, nil, s.getNextRetryTime(delivery.Attempts+1), delivery.Attempts+1)
	}

	// Get active secrets for signing
	secrets, err := s.repository.ListActiveSecrets(ctx, exec, webhook.Id)
	if err != nil {
		errMsg := fmt.Sprintf("failed to get webhook secrets: %s", err.Error())
		logger.ErrorContext(ctx, errMsg, "delivery_id", delivery.Id, "webhook_id", delivery.WebhookId)
		return s.repository.MarkWebhookDeliveryFailed(ctx, exec, delivery.Id, errMsg, nil, s.getNextRetryTime(delivery.Attempts+1), delivery.Attempts+1)
	}

	// Build request
	timestamp := time.Now().Unix()
	req, err := http.NewRequestWithContext(ctx, "POST", webhook.Url, bytes.NewReader(payload))
	if err != nil {
		errMsg := fmt.Sprintf("failed to create request: %s", err.Error())
		logger.ErrorContext(ctx, errMsg, "delivery_id", delivery.Id)
		return s.repository.MarkWebhookDeliveryFailed(ctx, exec, delivery.Id, errMsg, nil, s.getNextRetryTime(delivery.Attempts+1), delivery.Attempts+1)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Webhook-Signature", s.buildSignatureHeader(secrets, timestamp, payload))
	req.Header.Set("X-Webhook-Id", webhook.Id.String())
	req.Header.Set("X-Webhook-Event-Id", event.Id)
	req.Header.Set("X-Webhook-Event-Type", string(event.EventContent.Type))
	req.Header.Set("X-Webhook-Timestamp", fmt.Sprintf("%d", timestamp))

	// Use webhook-specific timeout or default
	timeout := 30 * time.Second
	if webhook.HttpTimeout != nil {
		timeout = time.Duration(*webhook.HttpTimeout) * time.Second
	}

	client := &http.Client{Timeout: timeout}

	// Send request
	logger.DebugContext(ctx, "Sending webhook", "delivery_id", delivery.Id, "url", webhook.Url)
	resp, err := client.Do(req)
	if err != nil {
		errMsg := fmt.Sprintf("failed to send request: %s", err.Error())
		logger.WarnContext(ctx, errMsg, "delivery_id", delivery.Id, "attempt", delivery.Attempts+1)
		return s.repository.MarkWebhookDeliveryFailed(ctx, exec, delivery.Id, errMsg, nil, s.getNextRetryTime(delivery.Attempts+1), delivery.Attempts+1)
	}
	defer resp.Body.Close()

	responseStatus := resp.StatusCode

	// Check response status
	if responseStatus >= 200 && responseStatus < 300 {
		logger.InfoContext(ctx, "Webhook delivered successfully",
			"delivery_id", delivery.Id,
			"status_code", responseStatus,
			"attempts", delivery.Attempts+1)
		return s.repository.MarkWebhookDeliverySuccess(ctx, exec, delivery.Id, responseStatus)
	}

	// Handle failure
	errMsg := fmt.Sprintf("webhook returned status %d", responseStatus)
	logger.WarnContext(ctx, "Webhook delivery failed",
		"delivery_id", delivery.Id,
		"status_code", responseStatus,
		"attempt", delivery.Attempts+1)

	return s.repository.MarkWebhookDeliveryFailed(ctx, exec, delivery.Id, errMsg, &responseStatus, s.getNextRetryTime(delivery.Attempts+1), delivery.Attempts+1)
}

// ValidateWebhookEndpoint validates that a webhook URL is reachable
func ValidateWebhookEndpoint(ctx context.Context, url string, timeout time.Duration) error {
	client := &http.Client{Timeout: timeout}

	// Send empty POST to verify endpoint accepts requests
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader([]byte("{}")))
	if err != nil {
		return errors.Wrap(models.BadParameterError, fmt.Sprintf("invalid webhook URL: %s", err.Error()))
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Webhook-Validation", "true")

	resp, err := client.Do(req)
	if err != nil {
		return errors.Wrap(models.BadParameterError, fmt.Sprintf("webhook endpoint unreachable: %s", err.Error()))
	}
	defer resp.Body.Close()

	// Accept any 2xx response as valid
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return errors.Wrap(models.BadParameterError,
			fmt.Sprintf("webhook endpoint returned status %d, expected 2xx", resp.StatusCode))
	}

	return nil
}
