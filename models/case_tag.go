package models

import "time"

type CaseTag struct {
	Id        string
	CaseId    string
	TagId     string
	CreatedAt time.Time
	DeletedAt *time.Time
}

type CreateCaseTagsAttributes struct {
	CaseId string
	TagIds []string
}
