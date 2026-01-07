package security

import (
	"errors"

	"github.com/checkmarble/marble-backend/models"
	"github.com/google/uuid"
)

type EnforceSecurityTags interface {
	EnforceSecurity
	ReadTag(tag models.Tag) error
	CreateTag(organizationId string) error
	UpdateTag(tag models.Tag) error
	DeleteTag(tag models.Tag) error
}

func (e *EnforceSecurityImpl) ReadTag(tag models.Tag) error {
	orgId, _ := uuid.Parse(tag.OrganizationId)
	return errors.Join(
		e.Permission(models.TAG_READ),
		e.ReadOrganization(orgId),
	)
}

func (e *EnforceSecurityImpl) CreateTag(organizationId string) error {
	orgId, _ := uuid.Parse(organizationId)
	return errors.Join(
		e.Permission(models.TAG_CREATE),
		e.ReadOrganization(orgId),
	)
}

func (e *EnforceSecurityImpl) UpdateTag(tag models.Tag) error {
	orgId, _ := uuid.Parse(tag.OrganizationId)
	return errors.Join(
		e.Permission(models.TAG_UPDATE),
		e.ReadOrganization(orgId),
	)
}

func (e *EnforceSecurityImpl) DeleteTag(tag models.Tag) error {
	orgId, _ := uuid.Parse(tag.OrganizationId)
	return errors.Join(
		e.Permission(models.TAG_DELETE),
		e.ReadOrganization(orgId),
	)
}
