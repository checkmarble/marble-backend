package api

import (
	"fmt"
	"net/http"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/usecases"
	"github.com/checkmarble/marble-backend/utils"
)

func usecasesWithCreds(r *http.Request, uc usecases.Usecases) *usecases.UsecasesWithCreds {
	ctx := r.Context()

	creds, found := utils.CredentialsFromCtx(ctx)
	if !found {
		panic("no credentials in context")
	}

	// marble admin can specify on which organization to operate
	// Ignore error, empty organizationId is fine, this is not the place to enforce security
	organizationId, _ := utils.OrganizationIdFromRequest(r)

	return &usecases.UsecasesWithCreds{
		Usecases:    uc,
		Credentials: creds,
		OrganizationIdOfContext: func() (string, error) {
			if organizationId == "" {
				return "", fmt.Errorf(
					"no OrganizationId for %s in this context. MarbleAdmin can specify one using 'organization-id' query param. %w",
					creds.ActorIdentityDescription(),
					models.BadParameterError,
				)
			}
			return organizationId, nil
		},
	}
}
