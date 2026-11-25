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
	// AutoAdded is true when the server added the object on its own as a
	// useful parent of a selected object (not an analyst choice).
	AutoAdded bool
	// ContextOnly: the object is context to read and cite but is never
	// declared on its own. True for an auto-added parent that is not a direct
	// candidate of the declaration; an auto-added direct candidate (decision
	// trigger, pivot, case entity) stays declarable.
	ContextOnly bool
	CreatedAt   time.Time
}

// SarReachedFrom records the object and link through which a selected object
// was reached. For an auto-added object it is display information only (« lié à
// X »): Table/ObjectId is the nearest selected object it was reached from when
// it was added and Link the last link followed, which behind an unselected
// intermediate table is not a link of Table. It never decides whether the
// object stays selected and must not be used to navigate.
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
