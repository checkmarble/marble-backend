package api

import (
	"net/http"

	"github.com/checkmarble/marble-backend/usecases"
	"github.com/checkmarble/marble-backend/utils"
)

func usecasesWithCreds(r *http.Request, uc usecases.Usecases) *usecases.UsecasesWithCreds {
	ctx := r.Context()

	creds, found := utils.CredentialsFromCtx(ctx)
	if !found {
		panic("no credentials in context")
	}

	return &usecases.UsecasesWithCreds{
		Usecases:    uc,
		Credentials: creds,
	}
}
