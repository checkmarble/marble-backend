package security

import (
	"github.com/checkmarble/marble-backend/models"
)

type EnforceSecurityAnnotation interface {
	DeleteAnnotation() error
}

type EnforceSecurityAnnotationImpl struct {
	EnforceSecurity
	Credentials models.Credentials
}

func (e *EnforceSecurityAnnotationImpl) DeleteAnnotation() error {
	return e.Permission(models.ANNOTATION_DELETE)
}
