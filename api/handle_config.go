package api

import (
	"net/http"

	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/usecases"
	"github.com/checkmarble/marble-backend/usecases/auth"
	"github.com/gin-gonic/gin"
)

func handleGetConfig(uc usecases.Usecases, cfg Configuration) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		licenseUsecase := uc.NewLicenseUsecase()
		versionUsecase := uc.NewVersionUsecase()

		signupUsecase := usecases.NewSignupUsecase(uc.NewExecutorFactory(),
			uc.Repositories.MarbleDbRepository,
			uc.Repositories.MarbleDbRepository,
		)

		migrationsRunForOrgs, hasAnOrganization, err := signupUsecase.HasAnOrganization(ctx)
		if presentError(ctx, c, err) {
			return
		}

		migrationsRunForUsers, hasAUser, err := signupUsecase.HasAUser(ctx)
		if presentError(ctx, c, err) {
			return
		}

		var oidcConfig *dto.ConfigAuthOidcDto

		if cfg.TokenProvider == auth.TokenProviderOidc {
			oidcConfig = &dto.ConfigAuthOidcDto{
				Issuer:      cfg.OidcConfig.Issuer,
				ClientId:    cfg.OidcConfig.ClientId,
				RedirectUri: cfg.OidcConfig.RedirectUri,
				Scopes:      cfg.OidcConfig.Scopes,
				ExtraParams: cfg.OidcConfig.ExtraParams,
			}
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
				Provider: cfg.TokenProvider.String(),
				Firebase: dto.ConfigAuthFirebaseDto{
					IsEmulator:   cfg.FirebaseConfig.EmulatorHost != "",
					EmulatorHost: cfg.FirebaseConfig.EmulatorHost,
					ProjectId:    dto.NewNullString(cfg.FirebaseConfig.ProjectId),
					ApiKey:       dto.NewNullString(cfg.FirebaseConfig.ApiKey),
					AuthDomain:   dto.NewNullString(cfg.FirebaseConfig.AuthDomain),
				},
				Oidc: oidcConfig,
			},
			Features: dto.ConfigFeaturesDto{
				Sso:     licenseUsecase.HasSsoEnabled(),
				Segment: !cfg.DisableSegment,
			},
		}

		c.JSON(http.StatusOK, out)
	}
}
