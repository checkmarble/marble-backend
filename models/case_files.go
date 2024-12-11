package models

import (
	"mime/multipart"
	"time"
)

type CaseFile struct {
	Id            string
	CaseId        string
	CreatedAt     time.Time
	BucketName    string
	FileReference string
	FileName      string
}

type CreateCaseFilesInput struct {
	CaseId string
	Files  []multipart.FileHeader
}

type CreateDbCaseFileInput struct {
	Id            string
	BucketName    string
	CaseId        string
	FileName      string
	FileReference string
}
