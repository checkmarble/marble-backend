package utils

import (
	"context"
	"fmt"
	. "marble/marble-backend/models"
)

type ContextKey int

const (
	ContextKeyCredentials ContextKey = iota
)

func CredentialsFromCtx(ctx context.Context) Credentials {

	creds, found := ctx.Value(ContextKeyCredentials).(*Credentials)

	if !found {
		panic(fmt.Errorf("Credentials not found in request context"))
	}

	return *creds
}

func OrgIDFromCtx(ctx context.Context) (id string, err error) {
	creds := CredentialsFromCtx(ctx)
	return creds.OrganizationId, nil
}
