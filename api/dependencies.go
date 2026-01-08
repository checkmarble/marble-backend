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

	var (
		idpTokenVerifier idp.TokenRepository
		tokenIssuer      string
	)

	switch conf.TokenProvider {
	case auth.TokenProviderFirebase:
		if len(optTokenVerifier) == 0 {
			firebaseApp := infra.InitializeFirebase(ctx, conf.FirebaseConfig.ProjectId)

			optTokenVerifier = append(optTokenVerifier, firebaseApp)
			firebaseAdmin = idp.NewAdminClient(conf.FirebaseConfig.ApiKey, firebaseApp, conf.MarbleAppUrl)
		}

		idpTokenVerifier = idp.NewFirebaseClient(conf.FirebaseConfig.ProjectId, optTokenVerifier[0])
		tokenIssuer = idpTokenVerifier.Issuer()
	case auth.TokenProviderOidc:
		oidcConfig, err := infra.InitializeOidc(ctx, conf.MarbleAppUrl)
		if err != nil {
			return dependencies{}, err
		}

		idpTokenVerifier = idp.NewOidcClient(oidcConfig, oidcConfig.Provider, oidcConfig.Issuer, oidcConfig.Verifier)
		tokenIssuer = idpTokenVerifier.Issuer()
	}

	jwtRepository := repositories.NewJWTRepository(tokenIssuer, signingKey)
	tokenValidator := token.NewValidator(database, jwtRepository)

	tokenHandler := auth.NewTokenHandler(
		auth.DefaultExtractor(),
		auth.NewVerifier(conf.TokenProvider, idpTokenVerifier, database, conf.OidcConfig.AllowedDomains),
		auth.NewGenerator(database, jwtRepository, time.Hour, clock.New()),
	)

	return dependencies{
		Authentication: utils.NewAuthentication(tokenValidator, conf.ScreeningIndexerToken),
		FirebaseAdmin:  firebaseAdmin,
		SegmentClient:  segmentClient,
		TokenHandler:   NewTokenHandler(tokenHandler),
	}, nil
}
