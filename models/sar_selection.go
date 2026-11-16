package models

import (
	"time"

	"github.com/google/uuid"
)

const (
	SarOriginKindDecision = "decision"
	SarOriginKindIngested = "ingested"
)

// SarSelectedObject is an object the analyst selected as input of a
// declaration. Snapshot is a frozen copy of the object's data taken at
// selection time: it never changes afterwards, even if the source is deleted.
type SarSelectedObject struct {
	Id            uuid.UUID
	DeclarationId uuid.UUID
	TableName     string
	ObjectId      string
	Snapshot      map[string]any
	Origin        SarSnapshotOrigin
	ReachedFrom   *SarReachedFrom // nil for a direct candidate
	CreatedAt     time.Time
}

// SarReachedFrom records the object and link through which a selected object
// was reached.
type SarReachedFrom struct {
	TableName string `json:"table"` //nolint:tagliatelle
	ObjectId  string `json:"object_id"`
	Link      string `json:"link"`
}

// SarSnapshotOrigin says where the snapshot data came from: the trigger object
// stored in a decision, or the ingested data read at selection time.
type SarSnapshotOrigin struct {
	Kind       string     `json:"kind"` // SarOriginKindDecision | SarOriginKindIngested
	DecisionId *uuid.UUID `json:"decision_id,omitempty"`
	ValidFrom  *time.Time `json:"valid_from,omitempty"`
}
