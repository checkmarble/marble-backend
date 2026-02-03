package models

import (
	"net/netip"
	"time"

	"github.com/google/uuid"
)

type CustomListKind int

const (
	CustomListUnknown CustomListKind = iota
	CustomListText
	CustomListCidrs
)

func CustomListKindFromString(s string) CustomListKind {
	switch s {
	case "text":
		return CustomListText
	case "cidrs":
		return CustomListCidrs
	default:
		return CustomListUnknown
	}
}

func (k CustomListKind) String() string {
	switch k {
	case CustomListText:
		return "text"
	case CustomListCidrs:
		return "cidrs"
	default:
		return "unknown"
	}
}

const VALUES_COUNT_LIMIT = 100 // Maximum count number of values to be returned when showing customs list information

type ValuesInfo struct {
	Count   int
	HasMore bool
}

type CustomList struct {
	Id             string
	OrganizationId uuid.UUID
	Name           string
	Description    string
	Kind           CustomListKind
	CreatedAt      time.Time
	UpdatedAt      time.Time
	DeletedAt      *time.Time
	ValuesCount    *ValuesInfo
}

type CustomListValue struct {
	Id           string
	CustomListId string
	Value        *string
	CidrValue    *netip.Prefix
	CreatedAt    time.Time
	DeletedAt    *time.Time
}

type CreateCustomListInput struct {
	Name           string
	Description    string
	Kind           CustomListKind
	OrganizationId uuid.UUID
}

type UpdateCustomListInput struct {
	Id          string
	Name        *string
	Description *string
}

type GetCustomListValuesInput struct {
	Id string
}

type AddCustomListValueInput struct {
	CustomListId string
	Value        string
}

type DeleteCustomListValueInput struct {
	Id           string
	CustomListId string
}

type BatchInsertCustomListValue struct {
	Id    string
	Value string
}

type BatchInsertCustomListValueResults struct {
	TotalExisting int
	TotalDeleted  int
	TotalCreated  int
}
