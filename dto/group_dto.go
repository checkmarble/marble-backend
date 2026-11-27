package dto

import (
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/google/uuid"
)

type CreateGroupInput struct {
	Name string `json:"name" binding:"required"`
}

type RenameGroupInput struct {
	Name string `json:"name" binding:"required"`
}

type ReplaceGroupMembersInput struct {
	Members []uuid.UUID `json:"members" binding:"required"`
}

type ReplaceGroupBindingsInput struct {
	Roles []RoleBinding `json:"roles" binding:"required"`
}

type Group struct {
	Id           uuid.UUID     `json:"id"`
	Name         string        `json:"name"`
	RoleBindings []RoleBinding `json:"roles"` //nolint:tagliatelle
	Members      []string      `json:"members"`
}

func AdaptGroup(group models.Group) Group {
	return Group{
		Id:           group.Id,
		Name:         group.Name,
		RoleBindings: pure_utils.Map(group.RoleBindings, AdaptRoleBinding),
		Members:      pure_utils.Map(group.MemberIds, func(id models.UserId) string { return string(id) }),
	}
}
