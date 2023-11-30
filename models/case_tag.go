package models

import "time"

type CaseTag struct {
	Id        string
	CaseId    string
	TagId     string
	CreatedAt time.Time
}

type CreateCaseTagAttributes struct {
	CaseId string
	TagId  string
}
