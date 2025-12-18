package utils

import (
	"fmt"
	"math/rand"

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

// TextToUUID is a function that converts a text to a UUID.
// The function is deterministic, so calling it several times with the same text will return the same UUID.
// It simplifies test writing by simplifying the creation of UUIDs and expected values.
// Usage:
//
//	uuid := utils.TextToUUID("organization-panoramix")
//	uuid2 := utils.TextToUUID("organization-panoramix")
//	assert.Equal(t, uuid, uuid2) // ✅ equal
func TextToUUID(text string) uuid.UUID {
	return uuid.NewSHA1(uuid.NameSpaceURL, []byte(text))
}

const nonceAlpha = "abcdefghijklmnopqrstuvwxyz"

func GenNonce(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = nonceAlpha[rand.Intn(len(nonceAlpha))]
	}
	return string(b)
}
