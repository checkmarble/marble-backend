package dto

import (
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/google/uuid"
)

type RoleBindingConditions struct {
	NotBefore *time.Time `json:"notBefore,omitempty"`
	NotAfter  *time.Time `json:"notAfter,omitempty"`
}

type RoleBinding struct {
	Id         uuid.UUID             `json:"id,omitempty"`
	Role       models.Role           `json:"role" binding:"required"`
	Conditions RoleBindingConditions `json:"conditions"`
}

func AdaptRoleBinding(binding models.RoleBinding) RoleBinding {
	return RoleBinding{
		Id:   binding.Id,
		Role: binding.Role,
		Conditions: RoleBindingConditions{
			NotBefore: binding.Conditions.NotBefore,
			NotAfter:  binding.Conditions.NotAfter,
		},
	}
}

func AdaptRoleBindingInput(binding RoleBinding) models.RoleBinding {
	result := models.RoleBinding{
		Id:   binding.Id,
		Role: binding.Role,
		Conditions: models.RoleBindingConditions{
			NotBefore: binding.Conditions.NotBefore,
			NotAfter:  binding.Conditions.NotAfter,
		},
	}
	return result
}
