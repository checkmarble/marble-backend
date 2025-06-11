package utils

import (
	"fmt"

	"github.com/checkmarble/marble-backend/models"
	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
)

func ValidateUuid(uuidParam string) error {
	_, err := uuid.Parse(uuidParam)
	if err != nil {
		err = fmt.Errorf("'%s' is not a valid UUID: %w", uuidParam, models.BadParameterError)
	}
	return err
}

func ByteUuid(str string) [16]byte {
	return [16]byte(uuid.MustParse(str))
}

func ParseSliceUUID(slice []string) ([]uuid.UUID, error) {
	parsed := make([]uuid.UUID, len(slice))
	for i, item := range slice {
		parsedItem, err := uuid.Parse(item)
		if err != nil {
			return nil, errors.Wrap(models.BadParameterError, "failed to parse UUID in slice")
		}
		parsed[i] = parsedItem
	}
	return parsed, nil
}
