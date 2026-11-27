package usecases

import (
	"github.com/checkmarble/marble-backend/infra"
	"github.com/checkmarble/marble-backend/models"
	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
)

func checkRoleBindingFeatureFlags(orgId uuid.UUID, binding models.RoleBinding) error {
	if binding.Role.IsCustom() && !infra.HasFeatureFlag(infra.CUSTOM_ROLES_FEATURE_FLAG, orgId) {
		return errors.Wrap(models.BadParameterError, "custom roles are not enabled for this organization")
	}

	return nil
}
