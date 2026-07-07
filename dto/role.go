package dto

import (
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/google/uuid"
)

type RolesAndPermissions struct {
	Roles       []Role   `json:"roles"`
	Permissions []string `json:"permissions"`
}

type Role struct {
	Id          uuid.UUID    `json:"id"`
	Slug        string       `json:"slug"`
	Name        string       `json:"name"`
	Permissions []Permission `json:"permissions"`
}

type RoleCreateInput struct {
	Slug string `json:"slug"`
	Name string `json:"name"`
}

type RoleGrant struct {
	Permissions []RoleGrantPermission `json:"permissions"`
}

type RoleGrantPermission struct {
	Name string `json:"name"`
}

type Permission struct {
	Id   uuid.UUID `json:"id"`
	Name string    `json:"name"`
}

func AdaptRole(role models.RbacRole) Role {
	return Role{
		Id:   role.Id,
		Slug: role.Slug,
		Name: role.Name,
		Permissions: pure_utils.Map(role.Permissions, func(p models.RbacPermission) Permission {
			return Permission{
				Id:   p.Id,
				Name: p.Name,
			}
		}),
	}
}
