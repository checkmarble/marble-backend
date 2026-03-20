package pure_utils

import (
	"github.com/google/uuid"
)

func NewId() uuid.UUID {
	return uuid.Must(uuid.NewV7())
}
