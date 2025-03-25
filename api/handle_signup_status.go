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

		hasAnOrganization, err := signupUc.HasAnOrganization(ctx)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		hasAUser, err := signupUc.HasAUser(ctx)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"has_an_organization": hasAnOrganization,
			"has_a_user":          hasAUser,
		})
	}
}
