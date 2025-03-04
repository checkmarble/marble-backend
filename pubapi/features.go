package pubapi

import (
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/usecases"
	"github.com/gin-gonic/gin"
)

func CheckFeatureAccess(c *gin.Context, uc *usecases.UsecasesWithCreds) bool {
	featureAccessReader := uc.NewFeatureAccessReader()

	features, err := featureAccessReader.GetOrganizationFeatureAccess(c.Request.Context(), uc.Credentials.OrganizationId)
	if err != nil {
		NewErrorResponse().WithError(err).Serve(c)
		return false
	}

	if !features.Sanctions.IsAllowed() {
		if features.Sanctions == models.MissingConfiguration {
			NewErrorResponse().WithError(ErrNotConfigured).Serve(c)
			return false
		}

		NewErrorResponse().WithError(ErrFeatureDisabled).Serve(c)
		return false
	}

	return true
}
