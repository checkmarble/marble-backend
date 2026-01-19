package pubapi

import (
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pubapi/types"
	"github.com/checkmarble/marble-backend/usecases"
	"github.com/gin-gonic/gin"
)

func CheckFeatureAccess(c *gin.Context, uc *usecases.UsecasesWithCreds) bool {
	featureAccessReader := uc.NewFeatureAccessReader()

	// Does not take into account access to AI features that are per-user - any per-user permissions do not make sense in the context of public API
	features, err := featureAccessReader.GetOrganizationFeatureAccess(c.Request.Context(), uc.Credentials.OrganizationId, nil)
	if err != nil {
		types.NewErrorResponse().WithError(err).Serve(c)
		return false
	}

	if !features.Sanctions.IsAllowed() {
		if features.Sanctions == models.MissingConfiguration {
			types.NewErrorResponse().WithError(types.ErrNotConfigured).Serve(c)
			return false
		}

		types.NewErrorResponse().WithError(types.ErrFeatureDisabled).Serve(c)
		return false
	}

	return true
}
