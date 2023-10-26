package dto

import (
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"
)

type CustomList struct {
	Id          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

func AdaptCustomListDto(list models.CustomList) CustomList {
	return CustomList{
		Id:          string(list.Id),
		Name:        list.Name,
		Description: list.Description,
		CreatedAt:   list.CreatedAt,
		UpdatedAt:   list.UpdatedAt,
	}
}

type CustomListWithValues struct {
	Id          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	CreatedAt   time.Time         `json:"createdAt"`
	UpdatedAt   time.Time         `json:"updatedAt"`
	Values      []CustomListValue `json:"values"`
}

type CustomListValue struct {
	Id    string `json:"id"`
	Value string `json:"value"`
}

func AdaptCustomListWithValuesDto(list models.CustomList, values []models.CustomListValue) CustomListWithValues {
	return CustomListWithValues{
		Id:          string(list.Id),
		Name:        list.Name,
		Description: list.Description,
		CreatedAt:   list.CreatedAt,
		UpdatedAt:   list.UpdatedAt,
		Values:      utils.Map(values, AdaptCustomListValueDto),
	}
}

func AdaptCustomListValueDto(listValue models.CustomListValue) CustomListValue {
	return CustomListValue{
		Id:    string(listValue.Id),
		Value: listValue.Value,
	}
}

type CreateCustomListBodyDto struct {
	Name        string `in:"path=name"`
	Description string `in:"path=description"`
}

type CreateCustomListInputDto struct {
	Body *CreateCustomListBodyDto `in:"body=json"`
}

type UpdateCustomListBodyDto struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type UpdateCustomListInputDto struct {
	CustomListID string                   `in:"path=customListId"`
	Body         *UpdateCustomListBodyDto `in:"body=json"`
}

type CreateCustomListValueBodyDto struct {
	Value string `json:"value"`
}
