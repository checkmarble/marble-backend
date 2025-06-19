package dto

import "github.com/google/uuid"

type UriUuid struct {
	_uuid uuid.UUID
}

func (u *UriUuid) Uuid() uuid.UUID {
	return u._uuid
}

func (u *UriUuid) UnmarshalParam(param string) error {
	parsed, err := uuid.Parse(param)
	if err != nil {
		return err
	}
	u._uuid = parsed
	return nil
}
