package api

import (
	"fmt"
	"log/slog"
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
	router   *gin.Engine
	usecases usecases.Usecases
}

func New(
	router *gin.Engine,
	port string,
	usecases usecases.Usecases,
	auth *Authentication,
	tokenHandler *TokenHandler,
	logger *slog.Logger,
) *http.Server {
	s := &API{
		router:   router,
		usecases: usecases,
	}

	s.routes(auth, tokenHandler, logger)

	return &http.Server{
		Addr:         fmt.Sprintf("0.0.0.0:%s", port),
		WriteTimeout: time.Second * 60,
		ReadTimeout:  time.Second * 15,
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
