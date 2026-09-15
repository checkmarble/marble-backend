package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type ScreeningSavedSearch struct {
	Id        uuid.UUID
	OrgId     uuid.UUID
	Provider  string
	Config    json.RawMessage
	CreatedAt time.Time
	DeletedAt *time.Time
}
