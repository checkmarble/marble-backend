package utils

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestTextToUUID(t *testing.T) {
	t.Run("deterministic behavior - same input produces same output", func(t *testing.T) {
		text := "organization-le-lion-ne-sassocie-pas-avec-le-cafard"

		uuid1 := TextToUUID(text)
		uuid2 := TextToUUID(text)

		assert.Equal(t, uuid1, uuid2, "same input should produce same UUID")
	})

	t.Run("different inputs produce different UUIDs", func(t *testing.T) {
		uuid1 := TextToUUID("organization-panoramix")
		uuid2 := TextToUUID("organization-numerobis")

		assert.NotEqual(t, uuid1, uuid2, "different inputs should produce different UUIDs")
	})

	t.Run("empty string produces valid UUID", func(t *testing.T) {
		result := TextToUUID("")

		assert.NotEqual(t, uuid.Nil, result, "empty string should produce a valid UUID")
		assert.Equal(t, result, TextToUUID(""), "empty string should be deterministic")
	})
}
