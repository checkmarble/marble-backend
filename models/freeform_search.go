package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type FreeformSearch struct {
	Id          uuid.UUID
	OrgId       uuid.UUID
	UserId      *uuid.UUID
	ApiKeyId    *uuid.UUID
	Provider    ScreeningProvider
	CreatedAt   time.Time
	SearchInput json.RawMessage
	Result      json.RawMessage
}
