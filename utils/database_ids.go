package utils

import (
	"github.com/google/uuid"
)

func NewPrimaryKey(organizationId string) string {
	// Output first 32 bits from the organizationId uuid, and the rest is random from a new uuid v4
	newUuid := uuid.New()
	orgIdAsUuid := uuid.MustParse(organizationId)

	var output uuid.UUID
	copy(output[:4], orgIdAsUuid[:4])
	copy(output[4:], newUuid[4:])

	return output.String()
}
