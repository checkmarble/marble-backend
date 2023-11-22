package dto

import (
	"time"

	"github.com/checkmarble/marble-backend/models"
)

type APICaseContributor struct {
	Id        string    `json:"id"`
	CaseId    string    `json:"case_id"`
	UserId    string    `json:"user_id"`
	CreatedAt time.Time `json:"created_at"`
}

func NewAPICaseContributor(caseContributor models.CaseContributor) APICaseContributor {
	return APICaseContributor{
		Id:        caseContributor.Id,
		CaseId:    caseContributor.CaseId,
		UserId:    caseContributor.UserId,
		CreatedAt: caseContributor.CreatedAt,
	}
}
