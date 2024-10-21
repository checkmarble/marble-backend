package api

import (
	"net/http"

	"github.com/checkmarble/marble-backend/usecases"
	"github.com/checkmarble/marble-backend/utils"
)

// Beware that r.Context() will return a different context if the route's timeout is over (and gin has moved
// on to the next handler). As a consequence, no long-running operations should be started before this function
// is called in the api handlers.
func usecasesWithCreds(r *http.Request, uc usecases.Usecases) *usecases.UsecasesWithCreds {
	creds, found := utils.CredentialsFromCtx(r.Context())
	if !found {
		panic("no credentials in context")
	}

	return &usecases.UsecasesWithCreds{
		Usecases:    uc,
		Credentials: creds,
	}
}
