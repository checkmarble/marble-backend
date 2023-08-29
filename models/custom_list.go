package models

import "time"

type CustomList struct {
	Id            string
	OrganizationId string
	Name          string
	Description   string
	CreatedAt     time.Time
	UpdatedAt     time.Time
	DeletedAt     *time.Time
}

type CustomListValue struct {
	Id           string
	CustomListId string
	Value        string
	CreatedAt    time.Time
	DeletedAt    *time.Time
}

type CreateCustomListInput struct {
	OrganizationId string
	Name          string
	Description   string
}

type UpdateCustomListInput struct {
	Id            string
	OrganizationId string
	Name          *string
	Description   *string
}

type DeleteCustomListInput struct {
	Id            string
	OrganizationId string
}

type GetCustomListValuesInput struct {
	Id            string
	OrganizationId string
}

type AddCustomListValueInput struct {
	OrganizationId string
	CustomListId  string
	Value         string
}

type DeleteCustomListValueInput struct {
	Id            string
	CustomListId  string
	OrganizationId string
}
