package models

import "github.com/google/uuid"

type Tenant struct {
	Id   uuid.UUID `json:"id"`
	Name string    `json:"name"`
}
