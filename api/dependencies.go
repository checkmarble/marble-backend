package api

import (
	"context"
	"crypto/rsa"

	"github.com/checkmarble/marble-backend/infra"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/repositories/firebase"
	"github.com/checkmarble/marble-backend/repositories/postgres"
	"github.com/checkmarble/marble-backend/usecases/token"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/segmentio/analytics-go/v3"
)

type dependencies struct {
	Authentication Authentication
	TokenHandler   TokenHandler
	SegmentClient  analytics.Client
}

func InitDependencies(ctx context.Context, conf Configuration, dbPool *pgxpool.Pool, signingKey *rsa.PrivateKey) dependencies {
	database := postgres.New(dbPool)

	auth := infra.InitializeFirebase(ctx)
	firebaseClient := firebase.New(auth)
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
