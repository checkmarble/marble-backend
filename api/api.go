package api

import (
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/usecases"
	"github.com/checkmarble/marble-backend/utils"
)

type API struct {
	router   *chi.Mux
	usecases usecases.Usecases
}

func New(router *chi.Mux, port string, usecases usecases.Usecases, auth *Authentication) *http.Server {
	s := &API{
		router:   router,
		usecases: usecases,
	}

	// Setup the routes
	s.routes(auth)

	// display routes for debugging
	s.displayRoutes()

	////////////////////////////////////////////////////////////
	// Setup a go http.Server
	////////////////////////////////////////////////////////////

	// create a go router instance
	srv := &http.Server{
		// address
		Addr: fmt.Sprintf("0.0.0.0:%s", port),

		// Good practice to set timeouts to avoid Slowloris attacks.
		WriteTimeout: time.Second * 15,
		ReadTimeout:  time.Second * 15,
		IdleTimeout:  time.Second * 60,

		// Instance of chi router
		Handler: router,
	}
	return srv
}

func (api *API) UsecasesWithCreds(r *http.Request) *usecases.UsecasesWithCreds {
	ctx := r.Context()

	creds := utils.CredentialsFromCtx(ctx)

	// marble admin can specify on which organization to operate
	// Ignore error, empty organizationId is fine, this is not the place to enforce security
	organizationId, _ := utils.OrganizationIdFromRequest(r)

	return &usecases.UsecasesWithCreds{
		Usecases:    api.usecases,
		Credentials: creds,
		Logger:      utils.LoggerFromContext(ctx),
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
		Context: ctx,
	}
}
