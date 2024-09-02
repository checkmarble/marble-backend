package dto

import (
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
)

type CustomList struct {
	Id               string    `json:"id"`
	Name             string    `json:"name"`
	Description      string    `json:"description"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
	CreatedAt_deprec time.Time `json:"createdAt"`
	UpdatedAt_deprec time.Time `json:"updatedAt"`
}

func AdaptCustomListDto(list models.CustomList) CustomList {
	return CustomList{
		Id:               list.Id,
		Name:             list.Name,
		Description:      list.Description,
		CreatedAt:        list.CreatedAt,
		UpdatedAt:        list.UpdatedAt,
		CreatedAt_deprec: list.CreatedAt,
		UpdatedAt_deprec: list.UpdatedAt,
	}
}

type CustomListWithValues struct {
	CustomList
	Values []CustomListValue `json:"values"`
}

type CustomListValue struct {
	Id    string `json:"id"`
	Value string `json:"value"`
}

func AdaptCustomListWithValuesDto(list models.CustomList, values []models.CustomListValue) CustomListWithValues {
	return CustomListWithValues{
		CustomList: AdaptCustomListDto(list),
		Values:     pure_utils.Map(values, AdaptCustomListValueDto),
	}
}

func AdaptCustomListValueDto(listValue models.CustomListValue) CustomListValue {
	return CustomListValue{
		Id:    listValue.Id,
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

type CreateCustomListValueBodyDto struct {
	Value string `json:"value"`
}
