package models

import "time"

type CustomList struct {
	Id          string
	OrgId       string
	Name        string
	Description string
	CreatedAt   time.Time
	UpdatedAt   time.Time
	DeletedAt *time.Time
}

type CustomListValue struct {
	Id        string
	CustomListId    string
	Value     string
	CreatedAt time.Time
	DeletedAt *time.Time
}

type CreateCustomListInput struct {
	OrgId       string
	Name        string
	Description string
}

type UpdateCustomListInput struct {
	Id          string
	OrgId       string
	Name        *string
	Description *string
}

type DeleteCustomListInput struct {
	Id    string
	OrgId string
}

type GetCustomListInput struct {
	Id    string
	OrgId string
}

type GetCustomListValuesInput struct {
	Id    string
	OrgId string
}

type AddCustomListValueInput struct {
	OrgId  string
	CustomListId string
	Value  string
}

type DeleteCustomListValueInput struct {
	Id     string
	CustomListId string
	OrgId  string
}
