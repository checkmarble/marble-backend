package models

import (
	"time"

	"github.com/google/uuid"
)

const CsvIngestionTotalTimeoutDefault = 12 * time.Hour

type UploadLogFilters struct {
	Status *UploadStatus
}

type UploadLog struct {
	Id             uuid.UUID
	OrganizationId uuid.UUID
	UserId         string
	FileName       string
	TableName      string
	UploadStatus   UploadStatus
	StartedAt      time.Time
	DeadlineAt     *time.Time
	FinishedAt     *time.Time
	LinesProcessed int
	RowsIngested   int
	// ByteOffset is the offset of the first CSV row not yet ingested. Zero means the file has not
	// been read yet; ingestion resumes from here when a previous attempt ran out of time.
	ByteOffset int64
	InputError *string
	Error      *string
}

// CsvIngestionOutcome tells the CsvIngestionWorker whether an upload log is done with or whether it
// ran out of time and should be resumed from its checkpoint on a later attempt. It exists so the
// ingestion usecase does not have to return river.JobSnooze itself.
type CsvIngestionOutcome int

const (
	// CsvIngestionCompleted means there is nothing left to do for this upload log, either because
	// it finished, failed, or was already in a terminal state.
	CsvIngestionCompleted CsvIngestionOutcome = iota
	// CsvIngestionIncomplete means the file was checkpointed part-way through and the job should be
	// snoozed so a later attempt picks up from the saved offset.
	CsvIngestionIncomplete
)

type UploadStatus string

const (
	UploadPending    UploadStatus = "pending"
	UploadProcessing UploadStatus = "processing"
	UploadSuccess    UploadStatus = "success"
	UploadFailure    UploadStatus = "failure"
)

func UploadStatusFrom(s string) UploadStatus {
	switch s {
	case "pending":
		return UploadPending
	case "success":
		return UploadSuccess
	case "failure":
		return UploadFailure
	case "processing":
		return UploadProcessing
	}
	return UploadPending
}

type UpdateUploadLogStatusInput struct {
	Id                           uuid.UUID
	UploadStatus                 UploadStatus
	CurrentUploadStatusCondition UploadStatus // for optimistic locking. Only rows matching this current status will be updated
	FinishedAt                   *time.Time
	DeadlineAt                   *time.Time
	NumRowsIngested              *int
	InputError                   *string
	Error                        *string
}
