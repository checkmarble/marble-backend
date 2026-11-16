package models

import (
	"time"

	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
)

// ErrSarJobAlreadyActive is returned when a generation job is already pending
// or running for the declaration.
var ErrSarJobAlreadyActive = errors.Wrap(ConflictError, "une génération est déjà en cours pour cette déclaration")

type SarJobKind string

const (
	SarJobKindGenerate SarJobKind = "generate"
	SarJobKindReview   SarJobKind = "review"
)

// SarJobStatus is the status of both jobs and tasks.
type SarJobStatus string

const (
	SarJobStatusPending SarJobStatus = "pending"
	SarJobStatusRunning SarJobStatus = "running"
	SarJobStatusDone    SarJobStatus = "done"
	SarJobStatusFailed  SarJobStatus = "failed"
)

type SarGenerationJob struct {
	Id            uuid.UUID
	DeclarationId uuid.UUID
	Kind          SarJobKind
	Status        SarJobStatus
	Input         []byte
	Error         string
	CreatedBy     *uuid.UUID
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type SarGenerationTask struct {
	Id         uuid.UUID
	JobId      uuid.UUID
	Kind       string
	TargetPath string
	Input      []byte
	InputHash  string
	DependsOn  []uuid.UUID
	Status     SarJobStatus
	Attempts   int
	Output     []byte
	Error      string
	// Trace is not loaded by the task listing (it can be large): use
	// GetSarGenerationTaskTrace.
	Trace      []byte
	StartedAt  *time.Time
	FinishedAt *time.Time
}

const (
	SarQuestionKindQuestion      = "question"
	SarQuestionKindReviewFinding = "review_finding"

	SarQuestionStatusOpen      = "open"
	SarQuestionStatusResolved  = "resolved"
	SarQuestionStatusDismissed = "dismissed"
)

type SarQuestion struct {
	Id            uuid.UUID
	DeclarationId uuid.UUID
	TaskId        *uuid.UUID
	Kind          string // "question" | "review_finding"
	Severity      string // review findings: "important" | "mineur"
	Paths         []string
	Text          string
	// Options is the JSON of the proposed answers, decoded by the usecase
	// (models does not import the sargen core package).
	Options   []byte
	Status    string
	CreatedAt time.Time
	ClosedAt  *time.Time
}
