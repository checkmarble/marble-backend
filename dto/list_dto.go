package dto

import (
	"marble/marble-backend/models"
	"time"
)

type List struct {
	Id          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

type ListValue struct {
	Id    string `json:"id"`
	Value string `json:"value"`
}

func AdaptListDto(list models.List) List {
	return List{
		Id:          string(list.Id),
		Name:        list.Name,
		Description: list.Description,
		CreatedAt:   list.CreatedAt,
		UpdatedAt:   list.UpdatedAt,
	}
}

func AdaptListValueDto(listValue models.ListValue) ListValue {
	return ListValue{
		Id:    string(listValue.Id),
		Value: listValue.Value,
	}
}

type CreateListBodyDto struct {
	Name        string `in:"path=name"`
	Description string `in:"path=description"`
}

type CreateListInputDto struct {
	Body *CreateListBodyDto `in:"body=json"`
}

type UpdateListBodyDto struct {
	Name        string `in:"path=name"`
	Description string `in:"path=description"`
}

type UpdateListInputDto struct {
	ListID string             `in:"path=listID"`
	Body   *UpdateListBodyDto `in:"body=json"`
}

type GetListInputDto struct {
	ListID string `in:"path=listID"`
}

type DeleteListInputDto struct {
	ListID string `in:"path=listID"`
}

type AddListValueInputDto struct {
	ListID string               `in:"path=listID"`
	Body   *AddListValueBodyDto `in:"body=json"`
}

type AddListValueBodyDto struct {
	Value string `in:"path=value"`
}

type DeleteListValueInputDto struct {
	ListID string                  `in:"path=listID"`
	Body   *DeleteListValueBodyDto `in:"body=json"`
}

type DeleteListValueBodyDto struct {
	Id string `in:"path=id"`
}
