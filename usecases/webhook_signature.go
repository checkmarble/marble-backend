package usecases

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/checkmarble/marble-backend/models"
)

// WebhookSignatureService generates Convoy-compatible HMAC-SHA256 signatures for webhook payloads.
type WebhookSignatureService struct{}

// Sign generates a Convoy-compatible signature header value.
// Always uses advanced mode with timestamp for replay protection.
// Format: t=<unix_timestamp>,v1=<sig1>,v2=<sig2>,...
//
// The signature is computed over: "<timestamp>,<payload>"
// Each active secret generates a separate vi= entry (v1, v2, v3, etc.).
func (s *WebhookSignatureService) Sign(payload []byte, secrets []models.NewWebhookSecret, timestamp int64) string {
	if len(secrets) == 0 {
		// No active secrets: return empty signature (respects API contract: it's valid base64 encoded bytes).
		// Authentication will fail on the receiving end.
		return fmt.Sprintf("t=%d,v1=", timestamp)
	}

	// Signature is computed over: "<timestamp>,<payload>"
	signedPayload := fmt.Sprintf("%d,%s", timestamp, string(payload))

	var sigs []string
	for i, secret := range secrets {
		sig := s.computeHMAC([]byte(signedPayload), secret.Value)
		sigs = append(sigs, fmt.Sprintf("v%d=%s", i+1, sig))
	}

	return fmt.Sprintf("t=%d,%s", timestamp, strings.Join(sigs, ","))
}

// computeHMAC computes HMAC-SHA256 and returns base64-encoded result.
func (s *WebhookSignatureService) computeHMAC(payload []byte, secret string) string {
	h := hmac.New(sha256.New, []byte(secret))
	h.Write(payload)
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
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
