package models

import "time"

type Tag struct {
	Id             string
	Target         TagTarget
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
	Target         TagTarget
	Name           string
}

type UpdateTagAttributes struct {
	Color string
	Name  string
	TagId string
}

type TagTarget string

const (
	TagTargetCase    TagTarget = "case"
	TagTargetObject  TagTarget = "object"
	TagTargetUnknown TagTarget = "unknown"
)

func TagTargetFromString(s string) TagTarget {
	switch s {
	case "case":
		return TagTargetCase
	case "object":
		return TagTargetObject
	default:
		return TagTargetUnknown
	}
}
