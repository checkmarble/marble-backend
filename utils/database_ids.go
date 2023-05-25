package utils

import (
	"github.com/google/uuid"
)

func NewPrimaryKey(orgId string) string {
	// Output first 32 bits from the orgId uuid, and the rest is random from a new uuid v4
	newUuid := uuid.New()
	orgIdAsUuid := uuid.MustParse(orgId)

	var output uuid.UUID
	copy(output[:4], orgIdAsUuid[:4])
	copy(output[4:], newUuid[4:])

	return output.String()
}
