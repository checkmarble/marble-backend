package models

import (
	"encoding/hex"

	"github.com/google/uuid"
)

type OrganizationShortId [4]byte

func NewOrganizationShortId(orgId string) OrganizationShortId {
	orgUuid := uuid.MustParse(orgId)
	return (OrganizationShortId)(orgUuid[:4])
}

func (shortId OrganizationShortId) String() string {
	var buf [8]byte
	hex.Encode(buf[:], shortId[:])
	return string(buf[:])
}
