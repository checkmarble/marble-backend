package api

import (
	"net/http"

	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/usecases"
	"github.com/checkmarble/marble-backend/usecases/auth"
	"github.com/gin-gonic/gin"
)

func handleCreateInitialOrganization(uc usecases.Usecases, cfg Configuration) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		var payload dto.CreateInitialOrg

		if err := c.ShouldBindBodyWithJSON(&payload); presentError(ctx, c, err) {
			return
		}

		onboardingUsecase := uc.NewOnboardingUsecase(cfg.TokenProviders.Has(auth.TokenProviderFirebase))

		if err := onboardingUsecase.CreateInitialOrganization(ctx, payload); presentError(ctx, c, err) {
			return
		}

		c.Status(http.StatusCreated)
	}
}
