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
			&uc.Repositories.MarbleDbRepository,
			&uc.Repositories.MarbleDbRepository,
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
			Version:         versionUsecase.ApiVersion,
			IsManagedMarble: licenseUsecase.IsManagedMarble(),
			Status: dto.ConfigStatusDto{
				Migrations: migrationsRunForOrgs && migrationsRunForUsers,
				HasOrg:     hasAnOrganization,
				HasUser:    hasAUser,
			},
			Urls: dto.ConfigUrlsDto{
				Marble:    dto.NewNullString(cfg.MarbleAppUrl),
				MarbleApi: dto.NewNullString(cfg.MarbleApiUrl),
				Metabase:  dto.NewNullString(cfg.MetabaseConfig.SiteUrl),
			},
			Auth: dto.ConfigAuthDto{
				Firebase: dto.ConfigAuthFirebaseDto{
					IsEmulator:   cfg.FirebaseConfig.EmulatorHost != "",
					EmulatorHost: cfg.FirebaseConfig.EmulatorHost,
					ProjectId:    dto.NewNullString(cfg.FirebaseConfig.ProjectId),
					ApiKey:       dto.NewNullString(cfg.FirebaseConfig.ApiKey),
					AuthDomain:   dto.NewNullString(cfg.FirebaseConfig.AuthDomain),
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
