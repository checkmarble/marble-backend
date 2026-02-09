package usecases

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/checkmarble/marble-backend/models"
)

// WebhookSignatureService generates Convoy-compatible HMAC-SHA256 signatures for webhook payloads.
type WebhookSignatureService struct{}

// Sign generates a Convoy-compatible signature header value.
// Always uses advanced mode with timestamp for replay protection.
// Format: t=<unix_timestamp>,v1=<sig1>,v1=<sig2>,...
//
// The signature is computed over: "<timestamp>,<payload>"
// Each active secret generates a separate v1= entry.
func (s *WebhookSignatureService) Sign(payload []byte, secrets []models.NewWebhookSecret, timestamp int64) string {
	if len(secrets) == 0 {
		return ""
	}

	// Signature is computed over: "<timestamp>,<payload>"
	signedPayload := fmt.Sprintf("%d,%s", timestamp, string(payload))

	var sigs []string
	for _, secret := range secrets {
		sig := s.computeHMAC([]byte(signedPayload), secret.Value)
		sigs = append(sigs, fmt.Sprintf("v1=%s", sig))
	}

	return fmt.Sprintf("t=%d,%s", timestamp, strings.Join(sigs, ","))
}

// computeHMAC computes HMAC-SHA256 and returns hex-encoded result.
func (s *WebhookSignatureService) computeHMAC(payload []byte, secret string) string {
	h := hmac.New(sha256.New, []byte(secret))
	h.Write(payload)
	return hex.EncodeToString(h.Sum(nil))
}

// GenerateSecret generates a cryptographically secure random secret for webhook signing.
// Returns a 32-byte hex-encoded string (64 characters).
func GenerateWebhookSecret() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("failed to generate random secret: %w", err)
	}
	return hex.EncodeToString(bytes), nil
}
