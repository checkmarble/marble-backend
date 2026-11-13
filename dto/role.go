package dto

import (
	"github.com/checkmarble/marble-backend/models"
)

type Role struct {
	Slug        string                     `json:"slug"`
	Name        string                     `json:"name"`
	Permissions []models.Permission        `json:"permissions"`
	Sections    map[models.AppSection]bool `json:"sections"`
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
		Sections:    models.SectionsVisibility(role.HiddenSections),
	}
}

type RoleSections struct {
	Role     models.Role                `json:"role" binding:"required"`
	Sections map[models.AppSection]bool `json:"sections" binding:"required"`
}
