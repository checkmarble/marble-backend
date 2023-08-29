package security

import (
	"errors"
	"marble/marble-backend/models"
)

type EnforceSecurityCustomList interface {
	EnforceSecurity
	ReadCustomList(customList models.CustomList) error
	CreateCustomList(organizationId string) error
}

type EnforceSecurityCustomListImpl struct {
	EnforceSecurity
	Credentials models.Credentials
}

func (e *EnforceSecurityCustomListImpl) ReadCustomList(customList models.CustomList) error {
	return errors.Join(
		e.Permission(models.DECISION_READ),
		e.ReadOrganization(customList.OrganizationId),
	)
}

func (e *EnforceSecurityCustomListImpl) CreateCustomList(organizationId string) error {
	return errors.Join(
		e.Permission(models.DECISION_CREATE),
		e.ReadOrganization(organizationId),
	)
}
