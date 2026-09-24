package dto

import (
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/google/uuid"
)

type User struct {
	UserId         string        `json:"user_id"`
	Email          string        `json:"email"`
	RoleBindings   []RoleBinding `json:"roles"`
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
		RoleBindings:   pure_utils.Map(user.RoleBindings, AdaptRoleBinding),
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
	RoleBindings   []RoleBinding `json:"roles"`
	OrganizationId uuid.UUID     `json:"organization_id"`
	FirstName      string        `json:"first_name"`
	LastName       string        `json:"last_name"`
}

type UpdateUser struct {
	Email        *string        `json:"email"`
	RoleBindings *[]RoleBinding `json:"roles"`
	FirstName    *string        `json:"first_name"`
	LastName     *string        `json:"last_name"`
}

func AdaptCreateUser(dto CreateUser) models.CreateUser {
	return models.CreateUser{
		Email:          dto.Email,
		RoleBindings:   pure_utils.Map(dto.RoleBindings, AdaptRoleBindingInput),
		OrganizationId: dto.OrganizationId,
		FirstName:      dto.FirstName,
		LastName:       dto.LastName,
	}
}

func AdaptUpdateUser(dto UpdateUser, userId string) models.UpdateUser {
	var updatedBindings *[]models.RoleBinding
	if dto.RoleBindings != nil {
		bindings := pure_utils.Map(*dto.RoleBindings, AdaptRoleBindingInput)
		updatedBindings = &bindings
	}

	return models.UpdateUser{
		UserId:       userId,
		Email:        dto.Email,
		RoleBindings: updatedBindings,
		FirstName:    dto.FirstName,
		LastName:     dto.LastName,
	}
}
