package dto

import (
	"time"

	"github.com/checkmarble/marble-backend/models"
)

type APICaseTag struct {
	Id        string    `json:"id"`
	CaseId    string    `json:"case_id"`
	TagId     string    `json:"tag_id"`
	CreatedAt time.Time `json:"created_at"`
}

func NewAPICaseTag(t models.CaseTag) APICaseTag {
	apiCaseTag := APICaseTag{
		Id:        t.Id,
		CaseId:    t.CaseId,
		TagId:     t.TagId,
		CreatedAt: t.CreatedAt,
	}

	return apiCaseTag
}

type CreateCaseTagBody struct {
	TagId string `json:"tag_id" binding:"required"`
}
