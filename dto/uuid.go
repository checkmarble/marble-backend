package dto

import "github.com/google/uuid"

type UnmarshallingUuid struct {
	_uuid uuid.UUID
}

func (u *UnmarshallingUuid) Uuid() uuid.UUID {
	return u._uuid
}

func (u *UnmarshallingUuid) UnmarshalParam(param string) error {
	parsed, err := uuid.Parse(param)
	if err != nil {
		return err
	}
	u._uuid = parsed
	return nil
}
