package security

import (
	"github.com/checkmarble/marble-backend/models"
	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
)

type EnforceSecurityAnnotation interface {
	EnforceSecurity
	DeleteAnnotation() error
	WriteAnnotation(orgId uuid.UUID) error
}

type EnforceSecurityAnnotationImpl struct {
	EnforceSecurity
	Credentials models.Credentials
}

func (e *EnforceSecurityAnnotationImpl) DeleteAnnotation() error {
	return e.Permission(models.ANNOTATION_DELETE)
}

func (e *EnforceSecurityAnnotationImpl) WriteAnnotation(orgId uuid.UUID) error {
	return errors.Join(e.Permission(models.ANNOTATION_WRITE),
		e.ReadOrganization(orgId))
}
