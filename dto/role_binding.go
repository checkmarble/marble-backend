package dto

import (
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/google/uuid"
)

type RoleBindingConditions struct {
	NotBefore        *time.Time             `json:"not_before,omitempty"`
	NotAfter         *time.Time             `json:"not_after,omitempty"`
	DayOfWeek        *[]time.Weekday        `json:"day_of_week,omitempty"`
	TimeOfDay        *models.TimeOfDayRange `json:"time_of_day,omitempty"`
	Networks         []SubnetDto            `json:"networks,omitempty"`
	UsedSecondFactor *bool                  `json:"used_second_factor,omitempty"`
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
			NotBefore:        binding.Conditions.NotBefore,
			NotAfter:         binding.Conditions.NotAfter,
			DayOfWeek:        binding.Conditions.DayOfWeek,
			TimeOfDay:        binding.Conditions.TimeOfDay,
			Networks:         binding.Conditions.Networks,
			UsedSecondFactor: binding.Conditions.UsedSecondFactor,
		},
	}
}

func AdaptRoleBindingInput(binding RoleBinding) models.RoleBinding {
	result := models.RoleBinding{
		Id:   binding.Id,
		Role: binding.Role,
		Conditions: models.RoleBindingConditions{
			NotBefore:        binding.Conditions.NotBefore,
			NotAfter:         binding.Conditions.NotAfter,
			DayOfWeek:        binding.Conditions.DayOfWeek,
			TimeOfDay:        binding.Conditions.TimeOfDay,
			Networks:         binding.Conditions.Networks,
			UsedSecondFactor: binding.Conditions.UsedSecondFactor,
		},
	}
	return result
}
