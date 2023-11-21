package dto

import (
	"time"

	"github.com/checkmarble/marble-backend/models"
)

type InboxDto struct {
	Id        string    `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Name      string    `json:"name"`
	Status    string    `json:"status"`
}

func AdaptInboxDto(i models.Inbox) InboxDto {
	return InboxDto{
		Id:        i.Id,
		CreatedAt: i.CreatedAt,
		UpdatedAt: i.UpdatedAt,
		Name:      i.Name,
		Status:    string(i.Status),
	}

}

type InboxUserDto struct {
	Id        string    `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Role      string    `json:"role"`
	InboxId   string    `json:"inbox_id"`
	UserId    string    `json:"user_id"`
}

func AdaptInboxUserDto(i models.InboxUser) InboxUserDto {
	return InboxUserDto{
		Id:        i.Id,
		CreatedAt: i.CreatedAt,
		UpdatedAt: i.UpdatedAt,
		Role:      string(i.Role),
		InboxId:   i.InboxId,
		UserId:    i.UserId,
	}
}
