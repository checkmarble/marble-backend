package dto

import (
	"time"

	"github.com/checkmarble/marble-backend/models"
)

type APICase struct {
	Id          string    `json:"id"`
	CreatedAt   time.Time `json:"created_at"`
	Description *string   `json:"description"`
	Name        string    `json:"name"`
	Status      string    `json:"status"`
}

func AdaptCaseDto(c models.Case) APICase {
	return APICase{
		Id:          c.Id,
		CreatedAt:   c.CreatedAt,
		Description: c.Description,
		Name:        c.Name,
		Status:      string(c.Status),
	}
}
