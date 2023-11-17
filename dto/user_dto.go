package dto

import "github.com/checkmarble/marble-backend/models"

type User struct {
	UserId         string `json:"user_id"`
	Email          string `json:"email"`
	Role           string `json:"role"`
	OrganizationId string `json:"organization_id"`
	FirstName      string `json:"first_name"`
	LastName       string `json:"last_name"`
}

func AdaptUserDto(user models.User) User {
	return User{
		UserId:         string(user.UserId),
		Email:          user.Email,
		Role:           user.Role.String(),
		OrganizationId: user.OrganizationId,
		FirstName:      user.FirstName,
		LastName:       user.LastName,
	}
}

type CreateUser struct {
	Email          string `json:"email"`
	Role           string `json:"role"`
	OrganizationId string `json:"organization_id"`
	FirstName      string `json:"first_name"`
	LastName       string `json:"last_name"`
}

func AdaptCreateUser(dto CreateUser) models.CreateUser {
	return models.CreateUser{
		Email:          dto.Email,
		Role:           models.RoleFromString(dto.Role),
		OrganizationId: dto.OrganizationId,
		FirstName:      dto.FirstName,
		LastName:       dto.LastName,
	}
}
