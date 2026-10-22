package dto

import (
	"github.com/checkmarble/marble-backend/models"
)

type Role struct {
	Slug        string              `json:"slug"`
	Name        string              `json:"name"`
	Permissions []models.Permission `json:"permissions"`
}

type RoleCreateInput struct {
	Slug string `json:"slug"`
	Name string `json:"name"`
}

type RoleGrant struct {
	Role        models.Role         `json:"role" binding:"required"`
	Permissions []models.Permission `json:"permissions"`
}

func AdaptRole(role models.RbacRole) Role {
	return Role{
		Slug:        role.Slug,
		Name:        role.Name,
		Permissions: role.Permissions,
	}
}
