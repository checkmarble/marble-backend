package models

import "time"

type CustomList struct {
	Id             string
	OrganizationId string
	Name           string
	Description    string
	CreatedAt      time.Time
	UpdatedAt      time.Time
	DeletedAt      *time.Time
}

type CustomListValue struct {
	Id           string
	CustomListId string
	Value        string
	CreatedAt    time.Time
	DeletedAt    *time.Time
}

type CreateCustomListInput struct {
	Name           string
	Description    string
	OrganizationId string
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
