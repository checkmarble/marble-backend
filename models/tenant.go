package models

import "github.com/google/uuid"

type Tenant struct {
	Id   uuid.UUID
	Name string
}
