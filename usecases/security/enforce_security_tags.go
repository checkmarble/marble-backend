package security

import (
	"errors"

	"github.com/checkmarble/marble-backend/models"
)

type EnforceSecurityTags interface {
	EnforceSecurity
	ReadTag(tag models.Tag) error
	CreateTag(organizationId string) error
	UpdateTag(tag models.Tag) error
	DeleteTag(tag models.Tag) error
}

func (e *EnforceSecurityImpl) ReadTag(tag models.Tag) error {
	return errors.Join(
		e.Permission(models.TAG_READ),
		e.ReadOrganization(tag.OrganizationId),
	)
}

func (e *EnforceSecurityImpl) CreateTag(organizationId string) error {
	return errors.Join(
		e.Permission(models.TAG_CREATE),
		e.ReadOrganization(organizationId),
	)
}

func (e *EnforceSecurityImpl) UpdateTag(tag models.Tag) error {
	return errors.Join(
		e.Permission(models.TAG_UPDATE),
		e.ReadOrganization(tag.OrganizationId),
	)
}

func (e *EnforceSecurityImpl) DeleteTag(tag models.Tag) error {
	return errors.Join(
		e.Permission(models.TAG_DELETE),
		e.ReadOrganization(tag.OrganizationId),
	)
}
