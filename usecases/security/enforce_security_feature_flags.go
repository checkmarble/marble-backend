package security

import (
	"github.com/checkmarble/marble-backend/models"
)

func (e *EnforceSecurityImpl) ReadFeatureFlags() error {
	return e.Permission(models.FEATURE_FLAGS_READ)
}

func (e *EnforceSecurityImpl) WriteFeatureFlags() error {
	return e.Permission(models.FEATURE_FLAGS_WRITE)
}
