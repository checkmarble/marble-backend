package api

import (
	"context"
	"crypto/rsa"

	"github.com/checkmarble/marble-backend/infra"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/repositories/firebase"
	"github.com/checkmarble/marble-backend/repositories/postgres"
	"github.com/checkmarble/marble-backend/usecases/token"

	"firebase.google.com/go/v4/auth"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/segmentio/analytics-go/v3"
)

type dependencies struct {
	Authentication Authentication
	TokenHandler   TokenHandler
	SegmentClient  analytics.Client
}

type tokenVerifier interface {
	VerifyIDToken(ctx context.Context, idToken string) (*auth.Token, error)
}

func InitDependencies(
	ctx context.Context,
	conf Configuration,
	dbPool *pgxpool.Pool,
	signingKey *rsa.PrivateKey,
	optTokenVerifier ...tokenVerifier,
) dependencies {
	database := postgres.New(dbPool)

	if len(optTokenVerifier) == 0 {
		optTokenVerifier = append(optTokenVerifier, infra.InitializeFirebase(ctx))
	}

	firebaseClient := firebase.New(optTokenVerifier[0])
	jwtRepository := repositories.NewJWTRepository(signingKey)
	tokenValidator := token.NewValidator(database, jwtRepository)
	tokenGenerator := token.NewGenerator(database, jwtRepository, firebaseClient, conf.TokenLifetimeMinute)
	segmentClient := analytics.New(conf.SegmentWriteKey)

	return dependencies{
		Authentication: NewAuthentication(tokenValidator),
		SegmentClient:  segmentClient,
		TokenHandler:   NewTokenHandler(tokenGenerator),
	}
}
