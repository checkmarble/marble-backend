package infra

import (
	"context"

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/auth"
	"github.com/cockroachdb/errors"
)

func InitializeFirebase(ctx context.Context, projectId string) *auth.Client {
	app, err := firebase.NewApp(ctx, &firebase.Config{ProjectID: projectId})
	if err != nil {
		panic(errors.Wrap(err, "error initializing app"))
	}

	client, err := app.Auth(ctx)
	if err != nil {
		panic(errors.Wrap(err, "error getting Auth client"))
	}

	return client
}
