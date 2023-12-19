package dto

import (
	"time"

	"github.com/checkmarble/marble-backend/models"
)

type APICaseFile struct {
	Id        string    `json:"id"`
	CaseId    string    `json:"case_id"`
	CreatedAt time.Time `json:"created_at"`
	FileName  string    `json:"file_name"`
}

func NewAPICaseFile(caseFile models.CaseFile) APICaseFile {
	return APICaseFile{
		Id:        caseFile.Id,
		CaseId:    caseFile.CaseId,
		CreatedAt: caseFile.CreatedAt,
		FileName:  caseFile.FileName,
	}
}
