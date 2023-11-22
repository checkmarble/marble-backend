package models

import "time"

type CaseContributor struct {
	Id        string
	CaseId    string
	UserId    string
	CreatedAt time.Time
}
