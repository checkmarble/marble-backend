package api

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/usecases"
	"github.com/checkmarble/marble-backend/utils"
)

type API struct {
	router        *gin.Engine
	usecases      usecases.Usecases
	marbleAppHost string
}

func New(
	router *gin.Engine,
	port string,
	marbleAppHost string,
	usecases usecases.Usecases,
	auth Authentication,
	tokenHandler TokenHandler,
) *http.Server {
	s := &API{
		router:        router,
		usecases:      usecases,
		marbleAppHost: marbleAppHost,
	}

	s.routes(auth, tokenHandler)

	return &http.Server{
		Addr:         fmt.Sprintf("0.0.0.0:%s", port),
		WriteTimeout: time.Second * 60,
		ReadTimeout:  time.Second * 60,
		IdleTimeout:  time.Second * 60,
		Handler:      h2c.NewHandler(router, &http2.Server{}),
	}
}

func (api *API) UsecasesWithCreds(r *http.Request) *usecases.UsecasesWithCreds {
	ctx := r.Context()

	creds, found := utils.CredentialsFromCtx(ctx)
	if !found {
		panic("no credentials in context")
	}

	// marble admin can specify on which organization to operate
	// Ignore error, empty organizationId is fine, this is not the place to enforce security
	organizationId, _ := utils.OrganizationIdFromRequest(r)

	return &usecases.UsecasesWithCreds{
		Usecases:    api.usecases,
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
