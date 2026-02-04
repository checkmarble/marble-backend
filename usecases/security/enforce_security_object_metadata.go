package security

import (
	"github.com/checkmarble/marble-backend/models"
	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
)

type EnforceSecurityObjectMetadata interface {
	EnforceSecurity
	ReadObjectMetadata(metadata models.ObjectMetadata) error
	WriteObjectMetadata(orgId uuid.UUID) error
}

type EnforceSecurityObjectMetadataImpl struct {
	EnforceSecurity
	Credentials models.Credentials
}

func (e *EnforceSecurityObjectMetadataImpl) ReadObjectMetadata(metadata models.ObjectMetadata) error {
	return errors.Join(
		e.Permission(models.OBJECT_METADATA_READ),
		e.ReadOrganization(metadata.OrgId),
	)
}

func (e *EnforceSecurityObjectMetadataImpl) WriteObjectMetadata(orgId uuid.UUID) error {
	return errors.Join(
		e.Permission(models.OBJECT_METADATA_WRITE),
		e.ReadOrganization(orgId),
	)
}
