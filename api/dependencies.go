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

type tokenVerifier interface {
	VerifyIDToken(ctx context.Context, idToken string) (*firebase.Token, error)
}

func InitDependencies(
	ctx context.Context,
	conf Configuration,
	dbPool *pgxpool.Pool,
	signingKey *rsa.PrivateKey,
	optTokenVerifier ...tokenVerifier,
) (dependencies, error) {
	database := postgres.New(dbPool)

	var firebaseAdmin idp.Adminer

	if len(optTokenVerifier) == 0 {
		firebaseApp := infra.InitializeFirebase(ctx, conf.FirebaseConfig.ProjectId)

		optTokenVerifier = append(optTokenVerifier, firebaseApp)
		firebaseAdmin = idp.NewAdminClient(conf.FirebaseConfig.ApiKey, firebaseApp, conf.MarbleAppUrl)
	}

	if conf.DisableSegment {
		conf.SegmentWriteKey = ""
	}
	segmentClient := analytics.New(conf.SegmentWriteKey)

	var (
		tokenVerifier idp.TokenRepository
		tokenIssuer   string
	)

	switch conf.TokenProvider {
	case auth.TokenProviderFirebase:
		tokenVerifier = idp.NewFirebaseClient(conf.FirebaseConfig.ProjectId, optTokenVerifier[0])
		tokenIssuer = tokenVerifier.Issuer()
	case auth.TokenProviderOidc:
		oidcConfig, err := infra.InitializeOidc(ctx)
		if err != nil {
			return dependencies{}, err
		}

		tokenVerifier = idp.NewOidcClient(oidcConfig.Issuer, oidcConfig.Verifier)
		tokenIssuer = tokenVerifier.Issuer()
	}

	jwtRepository := repositories.NewJWTRepository(tokenIssuer, signingKey)
	tokenValidator := token.NewValidator(database, jwtRepository)

	tokenHandler := auth.NewTokenHandler(
		auth.DefaultExtractor(),
		auth.NewVerifier(conf.TokenProvider, tokenVerifier),
		auth.NewGenerator(database, jwtRepository, time.Hour, clock.New()),
	)

	return dependencies{
		Authentication: utils.NewAuthentication(tokenValidator),
		FirebaseAdmin:  firebaseAdmin,
		SegmentClient:  segmentClient,
		TokenHandler:   NewTokenHandler(tokenHandler),
	}, nil
}
