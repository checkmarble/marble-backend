package models

import "time"

type List struct {
	Id          string
	OrgId       string
	Name        string
	Description string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type ListValue struct {
	Id        string
	ListId    string
	Value     string
	CreatedAt time.Time
	DeletedAt time.Time
}

type CreateListInput struct {
	OrgId       string
	Name        string
	Description *string
}

type UpdateListInput struct {
	Id          string
	OrgId       string
	Name        *string
	Description *string
}

type DeleteListInput struct {
	Id    string
	OrgId string
}

type GetListInput struct {
	Id    string
	OrgId string
}

type GetListValuesInput struct {
	Id    string
	OrgId string
}

type AddListValueInput struct {
	OrgId  string
	ListId string
	Value  string
}

type DeleteListValueInput struct {
	Id     string
	ListId string
	OrgId  string
}
