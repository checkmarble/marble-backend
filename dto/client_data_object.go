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
	NextCursorId *string `json:"next_cursor_id"` // TODO: also handle integer case
	HasNextCase  bool    `json:"has_next_page"`
}
