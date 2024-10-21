package api

import (
	"context"

	"github.com/checkmarble/marble-backend/usecases"
	"github.com/checkmarble/marble-backend/utils"
)

func usecasesWithCreds(ctx context.Context, uc usecases.Usecases) *usecases.UsecasesWithCreds {
	creds, found := utils.CredentialsFromCtx(ctx)
	if !found {
		panic("no credentials in context")
	}

	return &usecases.UsecasesWithCreds{
		Usecases:    uc,
		Credentials: creds,
	}
}
