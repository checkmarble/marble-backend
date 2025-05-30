package models

import (
	"mime/multipart"
	"time"
)

type SarStatus int

const (
	SarPending SarStatus = iota
	SarCompleted
	SarUnknown
)

func (s SarStatus) String() string {
	switch s {
	case SarPending:
		return "pending"
	case SarCompleted:
		return "completed"
	default:
		return "unknown"
	}
}

func SarStatusFromString(s string) SarStatus {
	switch s {
	case "pending":
		return SarPending
	case "completed":
		return SarCompleted
	default:
		return SarUnknown
	}
}

type SuspiciousActivityReport struct {
	Id         string
	ReportId   string
	CaseId     string
	Status     SarStatus
	Bucket     *string
	BlobKey    *string
	CreatedBy  string
	UploadedBy *string
	CreatedAt  time.Time
	DeletedAt  *time.Time
}

type SuspiciousActivityReportRequest struct {
	CaseId     string
	ReportId   *string
	Bucket     *string
	BlobKey    *string
	Status     *SarStatus
	File       *multipart.FileHeader
	CreatedBy  UserId
	UploadedBy *UserId
	DeletedAt  *time.Time
}
