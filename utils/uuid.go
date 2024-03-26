package utils

import (
	"fmt"

	"github.com/checkmarble/marble-backend/models"
	"github.com/google/uuid"
)

func ValidateUuid(uuidParam string) error {
	_, err := uuid.Parse(uuidParam)
	if err != nil {
		err = fmt.Errorf("'%s' is not a valid UUID: %w", uuidParam, models.BadParameterError)
	}
	return err
}
