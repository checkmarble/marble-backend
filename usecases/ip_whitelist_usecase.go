package usecases

import (
	"context"
	"net"
	"net/http"

	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/gin-gonic/gin"
)

type IpWhitelistUse int

const (
	IpWhitelistLogin IpWhitelistUse = iota
	IpWhitelistOther
)

type ipWhitelistRepository interface {
	GetOrganizationSubnets(ctx context.Context, exec repositories.Executor, orgId string) ([]net.IPNet, error)
}

type IpWhitelistUsecase struct {
	executorFactory executor_factory.ExecutorFactory
	repository      ipWhitelistRepository
}

func (uc IpWhitelistUsecase) Guard(use IpWhitelistUse) gin.HandlerFunc {
	return func(c *gin.Context) {
		var buf *utils.BufferResponseWriter

		// If the guard is used for initial login, we actually need to perform
		// the login in order to know the user's organization and fetch its
		// whitelist.
		//
		// We use a dummy writer in order to be able to intercept the reponse
		// after it has been written.
		if use == IpWhitelistLogin {
			buf = utils.NewBufferResponseWriter(c)

			c.Next()
		}

		// We abort if the login was unsuccessful.
		if c.IsAborted() {
			return
		}

		ctx := c.Request.Context()
		logger := utils.LoggerFromContext(ctx)
		creds, ok := utils.CredentialsFromCtx(ctx)

		if !ok {
			logger.WarnContext(ctx, "a request with no credentials entered the IP whitelisting middleware")
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}

		clientIp := net.ParseIP(c.Request.Header.Get("x-real-ip"))

		// Self-hosted users might not have set the header on their reverse
		// proxy, so we fail open if it is not set.
		if clientIp == nil {
			if use == IpWhitelistLogin {
				buf.Restore(c)
			}

			return
		}

		subnets, err := uc.repository.GetOrganizationSubnets(ctx, uc.executorFactory.NewExecutor(), creds.OrganizationId)

		// TODO: here we might want to separate those two predicates, fail close
		// on error but open on empty whitelist.
		if err != nil || len(subnets) == 0 {
			if use == IpWhitelistLogin {
				buf.Restore(c)
			}

			return
		}

		for _, subnet := range subnets {
			if subnet.Contains(clientIp) {
				c.Request = c.Request.WithContext(context.WithValue(c.Request.Context(), utils.ContextKeyClientIp, clientIp))

				// If this was used for login, we have the response data in our
				// temporary buffer, we restore it and copy the data over.
				if use == IpWhitelistLogin {
					buf.Restore(c)
				}

				return
			}
		}

		logger.WarnContext(ctx, "blocked request for failing IP whitelisting configuration",
			"ip", clientIp,
			"subnets", subnets)

		c.AbortWithStatus(http.StatusUnauthorized)
	}
}
