package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type ScreeningSavedSearch struct {
	Id        uuid.UUID
	OrgId     uuid.UUID
	Name      string
	Provider  string
	Config    json.RawMessage
	CreatedAt time.Time
	DeletedAt *time.Time
}
