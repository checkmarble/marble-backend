package models

import (
	"time"

	"github.com/google/uuid"
)

type UserUnavailability struct {
	Id        uuid.UUID
	OrgId     uuid.UUID
	UserId    uuid.UUID
	FromDate  time.Time
	UntilDate time.Time
	CreatedAt time.Time
	UpdatedAt time.Time
}
