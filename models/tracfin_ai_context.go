package models

import (
	"encoding/json"
	"github.com/google/uuid"
	"time"
)

type TracfinAISource struct {
	ID      string          `json:"id"`
	Kind    string          `json:"kind"`
	Locator string          `json:"locator"`
	SHA256  string          `json:"sha256"`
	Content json.RawMessage `json:"content"`
}
type TracfinAIDiagnostic struct {
	SourceID string `json:"source_id,omitempty"`
	Code     string `json:"code"`
	Message  string `json:"message"`
}
type TracfinAIContext struct {
	ID              string                `json:"id"`
	CaseID          string                `json:"case_id"`
	OrganizationID  uuid.UUID             `json:"organization_id"`
	CollectedAt     time.Time             `json:"collected_at"`
	CaseFingerprint string                `json:"case_fingerprint"`
	Sources         []TracfinAISource     `json:"sources"`
	Complete        bool                  `json:"complete"`
	Diagnostics     []TracfinAIDiagnostic `json:"diagnostics"`
}
