package dto

import (
	"github.com/checkmarble/marble-backend/models"
	"github.com/google/uuid"
)

type Tenant struct {
	Id   uuid.UUID `json:"id"`
	Name string    `json:"name"`
}

func AdaptTenant(tenant models.Tenant) Tenant {
	return Tenant{Id: tenant.Id, Name: tenant.Name}
}
