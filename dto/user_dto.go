package dto

import (
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/google/uuid"
)

type User struct {
	UserId         string     `json:"user_id"`
	Email          string     `json:"email"`
	Role           string     `json:"role"`
	OrganizationId uuid.UUID  `json:"organization_id"`
	FirstName      string     `json:"first_name"`
	LastName       string     `json:"last_name"`
	Picture        string     `json:"picture"`
	DeletedAt      *time.Time `json:"deleted_at,omitempty"`
	TfaEnabled     *bool      `json:"tfa_enabled,omitempty"`
}

type TenantUser struct {
	UserId                string    `json:"user_id"`
	Email                 string    `json:"email"`
	OrganizationId        uuid.UUID `json:"organization_id"`
	FirstName             string    `json:"first_name"`
	LastName              string    `json:"last_name"`
	Picture               string    `json:"picture"`
	OrganizationGrantRole *string   `json:"organization_grant_role,omitempty"`
}

type TenantUsersResponse struct {
	Users []TenantUser `json:"users"`
}

func AdaptTenantUserDto(grant models.OrganizationUserGrant) TenantUser {
	user := grant.User
	dto := TenantUser{
		UserId:         string(user.UserId),
		Email:          user.Email,
		OrganizationId: user.OrganizationId,
		FirstName:      user.FirstName,
		LastName:       user.LastName,
		Picture:        user.Picture,
	}
	if grant.OrganizationGrantRole != nil {
		role := grant.OrganizationGrantRole.String()
		dto.OrganizationGrantRole = &role
	}
	return dto
}

func AdaptUserDto(user models.User) User {
	return User{
		UserId:         string(user.UserId),
		Email:          user.Email,
		Role:           user.Role.String(),
		OrganizationId: user.OrganizationId,
		FirstName:      user.FirstName,
		LastName:       user.LastName,
		Picture:        user.Picture,
		DeletedAt:      user.DeletedAt,
		TfaEnabled:     user.TfaEnabled,
	}
}

type CreateUser struct {
	Email          string    `json:"email"`
	Role           string    `json:"role"`
	OrganizationId uuid.UUID `json:"organization_id"`
	FirstName      string    `json:"first_name"`
	LastName       string    `json:"last_name"`
}

type UpdateUser struct {
	Email     *string `json:"email"`
	Role      *string `json:"role"`
	FirstName *string `json:"first_name"`
	LastName  *string `json:"last_name"`
}

type UserGrantDto struct {
	OrganizationId uuid.UUID `json:"organization_id"`
	TenantId       uuid.UUID `json:"tenant_id"`
	Role           string    `json:"role"`
}

// UserWithGrants is a user listed with `with_grants=true`.
type UserWithGrants struct {
	User
	Grants []UserGrantDto `json:"grants"`
}

func AdaptUserWithGrantsDto(user models.User, grants []models.Grant) UserWithGrants {
	return UserWithGrants{
		User:   AdaptUserDto(user),
		Grants: pure_utils.Map(grants, AdaptUserGrantDto),
	}
}

func AdaptUserGrantDto(grant models.Grant) UserGrantDto {
	return UserGrantDto{
		OrganizationId: grant.OrganizationId,
		TenantId:       grant.TenantId,
		Role:           grant.Role.String(),
	}
}

type ReplaceOrganizationGrant struct {
	Role string `json:"role" binding:"required"`
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

func AdaptUpdateUser(dto UpdateUser, userId string) models.UpdateUser {
	var updatedRole *models.Role
	if dto.Role != nil {
		new := models.RoleFromString(*dto.Role)
		updatedRole = &new
	}

	return models.UpdateUser{
		UserId:    userId,
		Email:     dto.Email,
		Role:      updatedRole,
		FirstName: dto.FirstName,
		LastName:  dto.LastName,
	}
}
