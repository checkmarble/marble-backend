package api

import (
	"context"
	"crypto/rsa"
	"time"

	"github.com/checkmarble/marble-backend/infra"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/repositories/clock"
	"github.com/checkmarble/marble-backend/repositories/idp"
	"github.com/checkmarble/marble-backend/repositories/postgres"
	"github.com/checkmarble/marble-backend/usecases/auth"
	"github.com/checkmarble/marble-backend/usecases/token"
	"github.com/checkmarble/marble-backend/utils"

	firebase "firebase.google.com/go/v4/auth"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/segmentio/analytics-go/v3"
)

type dependencies struct {
	Authentication utils.Authentication
	FirebaseAdmin  idp.Adminer
	TokenHandler   TokenHandler
	SegmentClient  analytics.Client
}

type idpTokenVerifier interface {
	VerifyIDToken(ctx context.Context, idToken string) (*firebase.Token, error)
}

func InitDependencies(
	ctx context.Context,
	conf Configuration,
	dbPool *pgxpool.Pool,
	signingKey *rsa.PrivateKey,
	optTokenVerifier ...idpTokenVerifier,
) (dependencies, error) {
	database := postgres.New(dbPool)

	var firebaseAdmin idp.Adminer

	if conf.DisableSegment {
		conf.SegmentWriteKey = ""
	}
	segmentClient := analytics.New(conf.SegmentWriteKey)

	verifiers := make(map[string]idp.TokenRepository)
	if conf.TokenProviders.Has(auth.TokenProviderFirebase) {
		if len(optTokenVerifier) == 0 {
			firebaseApp := infra.InitializeFirebase(ctx, conf.FirebaseConfig.ProjectId)

			optTokenVerifier = append(optTokenVerifier, firebaseApp)
			firebaseAdmin = idp.NewAdminClient(conf.FirebaseConfig.ApiKey, firebaseApp, conf.MarbleAppUrl)
		}

		firebaseVerifier := idp.NewFirebaseClient(conf.FirebaseConfig.ProjectId, optTokenVerifier[0])
		verifiers[firebaseVerifier.Issuer()] = firebaseVerifier
	}

	if conf.TokenProviders.Has(auth.TokenProviderOidc) {
		oidcVerifier := idp.NewOidcClient(conf.OidcConfig, conf.OidcConfig.Provider, conf.OidcConfig.Issuer, conf.OidcConfig.Verifier)
		verifiers[oidcVerifier.Issuer()] = oidcVerifier
	}

	issuers := make([]string, 0, len(verifiers))
	for issuer := range verifiers {
		issuers = append(issuers, issuer)
	}

	jwtRepository := repositories.NewJWTRepositoryForIssuers(issuers, signingKey)
	tokenValidator := token.NewValidator(database, jwtRepository)

	tokenLifetime := time.Duration(conf.TokenLifetimeMinute) * time.Minute
	if tokenLifetime <= 0 {
		tokenLifetime = time.Hour
	}

	tokenHandler := auth.NewTokenHandler(
		auth.DefaultExtractor(),
		auth.NewVerifier(verifiers, database, conf.OidcConfig.Issuer, conf.OidcConfig.AllowedDomains),
		auth.NewGenerator(database, jwtRepository, tokenLifetime, clock.New()),
	)

	return dependencies{
		Authentication: utils.NewAuthentication(tokenValidator, conf.ScreeningIndexerToken),
		FirebaseAdmin:  firebaseAdmin,
		SegmentClient:  segmentClient,
		TokenHandler:   NewTokenHandler(tokenHandler),
	}, nil
}
