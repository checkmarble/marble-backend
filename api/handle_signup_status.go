package api

import (
	"net/http"

	"github.com/checkmarble/marble-backend/usecases"
	"github.com/gin-gonic/gin"
)

func handleSignupStatus(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		signupUc := usecases.NewSignupUsecase(uc.NewExecutorFactory(),
			uc.Repositories.OrganizationRepository,
			uc.Repositories.UserRepository,
		)

		migrationsRunForOrgs, hasAnOrganization, err := signupUc.HasAnOrganization(ctx)
		if presentError(ctx, c, err) {
			return
		}

		migrationsRunForUsers, hasAUser, err := signupUc.HasAUser(ctx)
		if presentError(ctx, c, err) {
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"migrations_run":      migrationsRunForOrgs && migrationsRunForUsers,
			"has_an_organization": hasAnOrganization,
			"has_a_user":          hasAUser,
		})
	}
}
