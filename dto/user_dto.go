package dto

import (
	"time"

	"github.com/checkmarble/marble-backend/models"
)

type User struct {
	UserId         string     `json:"user_id"`
	Email          string     `json:"email"`
	Role           string     `json:"role"`
	OrganizationId string     `json:"organization_id"`
	PartnerId      *string    `json:"partner_id,omitempty"`
	FirstName      string     `json:"first_name"`
	LastName       string     `json:"last_name"`
	DeletedAt      *time.Time `json:"deleted_at,omitempty"`
}

func AdaptUserDto(user models.User) User {
	return User{
		UserId:         string(user.UserId),
		Email:          user.Email,
		Role:           user.Role.String(),
		OrganizationId: user.OrganizationId,
		PartnerId:      user.PartnerId,
		FirstName:      user.FirstName,
		LastName:       user.LastName,
		DeletedAt:      user.DeletedAt,
	}
}

type CreateUser struct {
	Email          string  `json:"email"`
	Role           string  `json:"role"`
	OrganizationId string  `json:"organization_id"`
	PartnerId      *string `json:"partner_id"`
	FirstName      string  `json:"first_name"`
	LastName       string  `json:"last_name"`
}

type UpdateUser struct {
	Email     string `json:"email"`
	Role      string `json:"role"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
}

func AdaptCreateUser(dto CreateUser) models.CreateUser {
	return models.CreateUser{
		Email:          dto.Email,
		Role:           models.RoleFromString(dto.Role),
		OrganizationId: dto.OrganizationId,
		PartnerId:      dto.PartnerId,
		FirstName:      dto.FirstName,
		LastName:       dto.LastName,
	}
}
