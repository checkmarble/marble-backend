package models

import (
	"encoding/json"
	"time"
)

// TracfinDecimal is an exact decimal lexeme, never a binary floating point value.
type TracfinDecimal string

type TracfinFieldRef struct {
	// ParentValueID identifies the scalar owner of an attribute. It also
	// localizes a missing attribute, for which no attribute ValueID exists.
	ParentValueID string `json:"parent_value_id,omitempty"`
	ValueID       string `json:"value_id,omitempty"`
	BlockID       string `json:"block_id"`
	FieldID       string `json:"field_id"`
}

// Text and Reference are distinct even when free text consists only of digits.
// A missing map entry is absent. An explicit empty string, false or zero remains present.
// Cleared records an explicit user decision and prevents automatic refill.
// ParentValueID identifies the owning scalar occurrence for attributes of
// simple content, e.g. nationalite/@type. It must name a value of the catalog
// field's immediate scalar parent in the same block. It is empty for elements
// and attributes directly on a block. Ownership never depends on array position;
// deleting a scalar must also remove its owned attributes (editing validates it).
type TracfinValue struct {
	ParentValueID string            `json:"parent_value_id,omitempty"`
	ID            string            `json:"id"`
	Kind          string            `json:"kind"` // text, boolean, decimal, reference, cleared
	Text          *string           `json:"text,omitempty"`
	Boolean       *bool             `json:"boolean,omitempty"`
	Decimal       *TracfinDecimal   `json:"decimal,omitempty"`
	Reference     *TracfinReference `json:"reference,omitempty"`
	Origin        *TracfinOrigin    `json:"origin,omitempty"`
	NeedsReview   bool              `json:"needs_review,omitempty"`
}

type TracfinReference struct {
	BlockID       string `json:"block_id"`
	PersonBlockID string `json:"person_block_id,omitempty"`
}

// TracfinBlock instances are restricted to manifest block IDs and their exact
// parent/child compatibility. Common XSD types share this representation; this
// is not an arbitrary XML node. Children retain their stable instance identities.
type TracfinBlock struct {
	ID             string                    `json:"id"`
	CatalogID      string                    `json:"catalog_id"`
	Fields         map[string][]TracfinValue `json:"fields,omitempty"`
	Children       []TracfinBlock            `json:"children,omitempty"`
	ActiveBranches map[string]string         `json:"active_branches,omitempty"`
	Selection      *TracfinSelection         `json:"selection,omitempty"`
	Attachment     *TracfinAttachment        `json:"attachment,omitempty"`
}

type TracfinDocument struct {
	Root TracfinBlock `json:"root"` // fixed catalog ID /ds
}

type TracfinDecisionSource struct {
	DecisionID     string          `json:"decision_id" db:"id"`
	HistoricalType string          `json:"historical_type" db:"trigger_object_type"`
	Snapshot       json.RawMessage `json:"snapshot" db:"trigger_object"`
}

type TracfinDiscriminator struct {
	Field  string   `json:"field"`
	Values []string `json:"values"`
}

type TracfinMappingSource struct {
	UnitOperation  bool              `json:"unit_operation,omitempty"`
	Kind           string            `json:"kind"`                  // ingested, decision_snapshot, settings
	EntityType     string            `json:"entity_type,omitempty"` // ingested roots: Person or Company
	Table          string            `json:"table,omitempty"`
	Field          string            `json:"field,omitempty"`
	HistoricalType string            `json:"historical_type,omitempty"`
	Path           []TracfinPathStep `json:"path,omitempty"`
}

type TracfinPathStep struct {
	ID        string `json:"id"`
	ParentID  string `json:"parent_id,omitempty"`
	Link      string `json:"link,omitempty"`
	Direction string `json:"direction,omitempty"`
	Table     string `json:"table"`
	RecordID  string `json:"record_id,omitempty"`
}

type TracfinSelection struct {
	MappingID      string            `json:"mapping_id"`
	Root           TracfinPathStep   `json:"root"`
	Bindings       []TracfinPathStep `json:"bindings,omitempty"`
	DecisionID     string            `json:"decision_id,omitempty"`
	HistoricalType string            `json:"historical_type,omitempty"`
	SnapshotField  string            `json:"snapshot_field,omitempty"`
}

type TracfinCodeMapping struct {
	Source string `json:"source"`
	Target string `json:"target"`
}

// Currency evidence is independent of the decimal value and unit conversion.
// No exchange rate or implicit currency conversion is supported.
type TracfinCurrencyEvidence struct {
	Field    string `json:"field,omitempty"`
	Constant string `json:"constant,omitempty"`
}

type TracfinMapping struct {
	Discriminator  *TracfinDiscriminator   `json:"discriminator,omitempty"`
	ID             string                  `json:"id"`
	TargetField    string                  `json:"target_field"`
	Source         TracfinMappingSource    `json:"source"`
	Codes          []TracfinCodeMapping    `json:"codes,omitempty"`
	Unit           string                  `json:"unit,omitempty"` // major or minor; no implicit conversion
	TargetCurrency string                  `json:"target_currency,omitempty"`
	Currency       TracfinCurrencyEvidence `json:"currency,omitempty"`
}

type TracfinConfiguration struct {
	OrgID          string           `json:"org_id"`
	Settings       TracfinDocument  `json:"settings"`
	Mappings       []TracfinMapping `json:"mappings"`
	ProfileVersion string           `json:"profile_version"`
	ProfileHash    string           `json:"profile_hash"`
	Revision       int64            `json:"revision"`
	CreatedAt      time.Time        `json:"created_at"`
	UpdatedAt      time.Time        `json:"updated_at"`
	UpdatedBy      string           `json:"updated_by"`
}

type TracfinSourceRef struct {
	Kind           string            `json:"kind"`
	Table          string            `json:"table,omitempty"`
	RecordID       string            `json:"record_id,omitempty"`
	Field          string            `json:"field,omitempty"`
	DecisionID     string            `json:"decision_id,omitempty"`
	HistoricalType string            `json:"historical_type,omitempty"`
	Path           []TracfinPathStep `json:"path,omitempty"`
}

type TracfinOrigin struct {
	SourceFingerprint string             `json:"source_fingerprint,omitempty"`
	Source            string             `json:"source"` // ingested, decision_snapshot, settings, manual, calculated
	MappingVersion    string             `json:"mapping_version,omitempty"`
	Sources           []TracfinSourceRef `json:"sources,omitempty"`
	Resolution        string             `json:"resolution"` // resolved, conflict, unavailable, cleared
}

type TracfinDiagnostic struct {
	Sources  []TracfinSourceRef `json:"sources,omitempty"`
	Code     string             `json:"code"`
	Layer    string             `json:"layer"`
	Severity string             `json:"severity"`
	Message  string             `json:"message"`
	Field    *TracfinFieldRef   `json:"field,omitempty"`
	RuleID   string             `json:"rule_id,omitempty"`
}

type TracfinValidationLayer struct {
	Status string `json:"status"` // passed, failed, unavailable
	Reason string `json:"reason,omitempty"`
}

type TracfinValidation struct {
	ProfileVersion string                            `json:"profile_version"`
	Layers         map[string]TracfinValidationLayer `json:"layers"`
	Diagnostics    []TracfinDiagnostic               `json:"diagnostics"`
}

// Populated by the server from a verified case file, never from client metadata.
type TracfinAttachment struct {
	SourceFileID string `json:"source_file_id"`
	CaseID       string `json:"case_id"`
	BlobKey      string `json:"blob_key"`
	SHA256       string `json:"sha256"`
	Size         int64  `json:"size"`
}

type TracfinExport struct {
	ID             string            `json:"id"`
	DraftRevision  int64             `json:"draft_revision"`
	ProfileVersion string            `json:"profile_version"`
	SHA256         string            `json:"sha256"`
	Size           int64             `json:"size"`
	BlobKey        string            `json:"blob_key"`
	Validation     TracfinValidation `json:"validation"`
	CreatedAt      time.Time         `json:"created_at"`
}

type TracfinDraft struct {
	Diagnostics   []TracfinDiagnostic  `json:"diagnostics,omitempty"`
	CaseID        string               `json:"case_id"`
	OrgID         string               `json:"org_id"`
	Document      TracfinDocument      `json:"document"`
	Configuration TracfinConfiguration `json:"configuration"`
	Revision      int64                `json:"revision"`
	LastExport    *TracfinExport       `json:"last_export,omitempty"`
	CreatedAt     time.Time            `json:"created_at"`
	UpdatedAt     time.Time            `json:"updated_at"`
	UpdatedBy     string               `json:"updated_by"`
}

// API input types deliberately omit server-controlled scope and provenance.
type TracfinSettingInput struct {
	Field TracfinFieldRef   `json:"field"`
	Value TracfinFieldInput `json:"value"`
}

type TracfinConfigurationInput struct {
	ExpectedRevision int64                 `json:"expected_revision"`
	Settings         []TracfinSettingInput `json:"settings"`
	Mappings         []TracfinMapping      `json:"mappings"`
}

type TracfinFieldInput struct {
	// Same ownership contract as TracfinValue; checked when applying changes.
	ParentValueID string            `json:"parent_value_id,omitempty"`
	Kind          string            `json:"kind"`
	Text          *string           `json:"text,omitempty"`
	Boolean       *bool             `json:"boolean,omitempty"`
	Decimal       *TracfinDecimal   `json:"decimal,omitempty"`
	Reference     *TracfinReference `json:"reference,omitempty"`
}

type TracfinChangeInput struct {
	CandidateID    string             `json:"candidate_id,omitempty"`
	MappingVersion string             `json:"mapping_version,omitempty"`
	Action         string             `json:"action"`
	Field          TracfinFieldRef    `json:"field"`
	Value          *TracfinFieldInput `json:"value,omitempty"`
	BlockID        string             `json:"block_id,omitempty"`
	ParentBlockID  string             `json:"parent_block_id,omitempty"`
	CatalogID      string             `json:"catalog_id,omitempty"`
	Selection      *TracfinSelection  `json:"selection,omitempty"`
	Relation       *TracfinReference  `json:"relation,omitempty"`
	SourceFileID   string             `json:"source_file_id,omitempty"`
	ProposalID     string             `json:"proposal_id,omitempty"`
}

type TracfinDraftInput struct {
	ExpectedRevision int64                `json:"expected_revision"`
	Changes          []TracfinChangeInput `json:"changes"`
}
