package dto

import (
	"encoding/json"

	"github.com/checkmarble/marble-backend/models"
)

type ClientDataListResponse struct {
	Data       []models.ClientObjectDetail `json:"data"`
	Pagination ClientDataListPagination    `json:"pagination"`
}

func (c ClientDataListResponse) MarshalJSON() ([]byte, error) {
	if c.Data == nil {
		c.Data = make([]models.ClientObjectDetail, 0)
	}
	return json.Marshal(struct {
		Data       []models.ClientObjectDetail `json:"data"`
		Pagination ClientDataListPagination    `json:"pagination"`
	}{
		Data:       c.Data,
		Pagination: c.Pagination,
	})
}

type ClientDataListPagination struct {
	NextCursorId *string `json:"next_cursor_id"`
	HasNextCase  bool    `json:"has_next_page"`
}

type ClientDataListRequestBody struct {
	ExplorationOptions ExplorationOptions `json:"exploration_options"`
	CursorId           *string            `json:"cursor_id"`
	Limit              *int               `json:"limit" validate:"gt=0,lte=100"`
}

type ExplorationOptions struct {
	SourceTableName   string         `json:"source_table_name"`
	FilterFieldName   string         `json:"filter_field_name"`
	FilterFieldValue  StringOrNumber `json:"filter_field_value"`
	OrderingFieldName string         `json:"ordering_field_name"`
}

func AdaptClientDataListRequestBody(input ClientDataListRequestBody) models.ClientDataListRequestBody {
	m := models.ClientDataListRequestBody{
		ExplorationOptions: models.ExplorationOptions{
			SourceTableName: input.ExplorationOptions.SourceTableName,
			FilterFieldName: input.ExplorationOptions.FilterFieldName,
			FilterFieldValue: AdaptStringOrNumber(
				input.ExplorationOptions.FilterFieldValue),
			OrderingFieldName: input.ExplorationOptions.OrderingFieldName,
		},
		CursorId: input.CursorId,
	}

	if input.Limit != nil {
		m.Limit = *input.Limit
	} else {
		m.Limit = 100
	}

	return m
}

func AdaptClientDataListPaginationDto(input models.ClientDataListPagination) ClientDataListPagination {
	return ClientDataListPagination{
		NextCursorId: input.NextCursorId,
		HasNextCase:  input.HasNextPage,
	}
}
