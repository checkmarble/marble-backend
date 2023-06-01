package dto

import "marble/marble-backend/models"

type User struct {
	UserId         string `json:"user_id"`
	Email          string `json:"email"`
	Role           string `json:"role"`
	OrganizationId string `json:"organization_id"`
}

func AdaptUserDto(user models.User) User {
	return User{
		UserId:         string(user.UserId),
		Email:          user.Email,
		Role:           user.Role.String(),
		OrganizationId: user.OrganizationId,
	}
}

type PostCreateUser struct {
	Body *CreateUser `in:"body=json"`
}

type CreateUser struct {
	Email          string `json:"email"`
	Role           string `json:"role"`
	OrganizationId string `json:"organization_id"`
}

func AdaptCreateUser(dto PostCreateUser) models.CreateUser {
	return models.CreateUser{
		Email:          dto.Body.Email,
		Role:           models.RoleFromString(dto.Body.Role),
		OrganizationId: dto.Body.OrganizationId,
	}
}
