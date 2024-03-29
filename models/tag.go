package models

import "time"

type Tag struct {
	Id             string
	Name           string
	Color          string
	OrganizationId string
	CreatedAt      time.Time
	UpdatedAt      time.Time
	DeletedAt      *time.Time
	CasesCount     *int
}

type CreateTagAttributes struct {
	Color          string
	OrganizationId string
	Name           string
}

type UpdateTagAttributes struct {
	Color string
	Name  string
	TagId string
}
