package models

import "time"

type UploadLog struct {
	Id             string
	OrganizationId string
	UserId         string
	FileName       string
	TableName      string
	UploadStatus   UploadStatus
	StartedAt      time.Time
	FinishedAt     *time.Time
	LinesProcessed int
}

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
	}
	return UploadPending
}

type UpdateUploadLogStatusInput struct {
	Id                           string
	UploadStatus                 UploadStatus
	CurrentUploadStatusCondition UploadStatus // for optimistic locking. Only rows matching this current status will be updated
	FinishedAt                   *time.Time
}
