package api

import (
	"context"

	"github.com/checkmarble/marble-backend/usecases"
	"github.com/checkmarble/marble-backend/utils"
)

func usecasesWithCreds(ctx context.Context, uc usecases.Usecaser) *usecases.UsecasesWithCreds {
	creds, found := utils.CredentialsFromCtx(ctx)
	if !found {
		panic("no credentials in context")
	}

	return &usecases.UsecasesWithCreds{
		Usecaser:    uc,
		Credentials: creds,
	}
}
