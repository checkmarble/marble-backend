package pure_utils

import (
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
)

type NullUUID struct {
	UUID  uuid.UUID
	Valid bool // Valid is true if UUID is not NULL
	Set   bool // Set is true if the value was present in JSON, even if it was null
}

func (u *NullUUID) UnmarshalJSON(data []byte) error {
	u.Set = true // Set to true if the value was present in JSON

	if string(data) == "null" {
		u.Valid = false
		u.UUID = uuid.Nil
		return nil
	}

	if err := json.Unmarshal(data, &u.UUID); err != nil {
		return fmt.Errorf("invalid UUID: %w", err)
	}

	u.Valid = true
	return nil
}

func (u NullUUID) Ptr() *uuid.UUID {
	if !u.Valid {
		return nil
	}
	return &u.UUID
}

func (u NullUUID) Value() any {
	if !u.Valid {
		return nil
	}
	return u.UUID
}

func NullUUIDFrom(u uuid.UUID) NullUUID {
	return NullUUID{
		UUID:  u,
		Valid: true,
		Set:   true,
	}
}
