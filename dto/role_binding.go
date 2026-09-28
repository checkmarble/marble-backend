package dto

import (
	"github.com/checkmarble/marble-backend/models"
	"github.com/google/uuid"
)

type RoleBinding struct {
	Id   uuid.UUID   `json:"id,omitempty"`
	Role models.Role `json:"role" binding:"required"`
}

func AdaptRoleBinding(binding models.RoleBinding) RoleBinding {
	return RoleBinding{
		Id:   binding.Id,
		Role: binding.Role,
	}
}

func AdaptRoleBindingInput(binding RoleBinding) models.RoleBinding {
	result := models.RoleBinding{
		Id:   binding.Id,
		Role: binding.Role,
	}
	return result
}
