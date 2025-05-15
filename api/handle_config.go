package api

import (
	"net/http"

	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/usecases"
	"github.com/gin-gonic/gin"
)

func handleGetConfig(uc usecases.Usecases, cfg Configuration) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		licenseUsecase := uc.NewLicenseUsecase()
		versionUsecase := uc.NewVersionUsecase()

		signupUsecase := usecases.NewSignupUsecase(uc.NewExecutorFactory(),
			uc.Repositories.OrganizationRepository,
			uc.Repositories.UserRepository,
		)

		migrationsRunForOrgs, hasAnOrganization, err := signupUsecase.HasAnOrganization(ctx)
		if presentError(ctx, c, err) {
			return
		}

		migrationsRunForUsers, hasAUser, err := signupUsecase.HasAUser(ctx)
		if presentError(ctx, c, err) {
			return
		}

		out := dto.ConfigDto{
			Version: versionUsecase.ApiVersion,
			Status: dto.ConfigStatusDto{
				Migrations: migrationsRunForOrgs && migrationsRunForUsers,
				HasOrg:     hasAnOrganization,
				HasUser:    hasAUser,
			},
			Urls: dto.ConfigUrlsDto{
				Marble:   cfg.MarbleAppUrl,
				Metabase: cfg.MetabaseConfig.SiteUrl,
			},
			Auth: dto.ConfigAuthDto{
				Firebase: dto.ConfigAuthFirebaseDto{
					IsEmulator:  cfg.FirebaseConfig.EmulatorUrl != "",
					EmulatorUrl: cfg.FirebaseConfig.EmulatorUrl,
					ProjectId:   cfg.FirebaseConfig.ProjectId,
					ApiKey:      cfg.FirebaseConfig.ApiKey,
					AuthDomain:  cfg.FirebaseConfig.AuthDomain,
				},
			},
			Features: dto.ConfigFeaturesDto{
				Sso:     licenseUsecase.HasSsoEnabled(),
				Segment: !cfg.DisableSegment,
			},
		}

		c.JSON(http.StatusOK, out)
	}
}
