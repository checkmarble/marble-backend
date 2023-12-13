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
}

type CreateCaseFileInput struct {
	CaseId string
	File   *multipart.FileHeader
}

type CreateDbCaseFileInput struct {
	BucketName    string
	CaseId        string
	FileReference string
	Id            string
}
