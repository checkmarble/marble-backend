package dto

import (
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/google/uuid"
)

type User struct {
	UserId         string        `json:"user_id"`
	Email          string        `json:"email"`
	Roles          []models.Role `json:"roles"`
	OrganizationId uuid.UUID     `json:"organization_id"`
	FirstName      string        `json:"first_name"`
	LastName       string        `json:"last_name"`
	Picture        string        `json:"picture"`
	DeletedAt      *time.Time    `json:"deleted_at,omitempty"`
	TfaEnabled     *bool         `json:"tfa_enabled,omitempty"`
}

func AdaptUserDto(user models.User) User {
	return User{
		UserId:         string(user.UserId),
		Email:          user.Email,
		Roles:          user.Roles,
		OrganizationId: user.OrganizationId,
		FirstName:      user.FirstName,
		LastName:       user.LastName,
		Picture:        user.Picture,
		DeletedAt:      user.DeletedAt,
		TfaEnabled:     user.TfaEnabled,
	}
}

type CreateUser struct {
	Email          string        `json:"email"`
	Roles          []models.Role `json:"roles"`
	OrganizationId uuid.UUID     `json:"organization_id"`
	FirstName      string        `json:"first_name"`
	LastName       string        `json:"last_name"`
}

type UpdateUser struct {
	Email     *string        `json:"email"`
	Roles     *[]models.Role `json:"roles"`
	FirstName *string        `json:"first_name"`
	LastName  *string        `json:"last_name"`
}

func AdaptCreateUser(dto CreateUser) models.CreateUser {
	return models.CreateUser{
		Email:          dto.Email,
		Roles:          dto.Roles,
		OrganizationId: dto.OrganizationId,
		FirstName:      dto.FirstName,
		LastName:       dto.LastName,
	}
}

func AdaptUpdateUser(dto UpdateUser, userId string) models.UpdateUser {
	var updatedRoles *[]models.Role
	if dto.Roles != nil {
		new := *dto.Roles
		updatedRoles = &new
	}

	return models.UpdateUser{
		UserId:    userId,
		Email:     dto.Email,
		Roles:     updatedRoles,
		FirstName: dto.FirstName,
		LastName:  dto.LastName,
	}
}
