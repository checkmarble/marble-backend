package tests

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"testing"

	v1 "github.com/checkmarble/marble-backend/pubapi/tests/specs/v1"
	"github.com/checkmarble/marble-backend/utils"
)

func TestPublicApi(t *testing.T) {
	for _, version := range []string{"v1beta", "v1"} {
		t.Run(fmt.Sprintf("Public API %s integration tests", version), func(it *testing.T) {
			ctx := context.Background()
			ctx = utils.StoreLoggerInContext(ctx, slog.New(slog.DiscardHandler))

			pg := setupPostgres(it, ctx)
			sock := setupApi(it, ctx, pg.MustConnectionString(ctx))

			client(t, sock, "", "").GET("/liveness").Expect().Status(http.StatusOK)
			client(t, sock, version, "invalidkey").GET("/decisions").Expect().Status(http.StatusUnauthorized)
			client(t, sock, version, "invalidkey").GET("/nothere").Expect().Status(http.StatusNotFound)

			v1.PublicApiV1(t, client(t, sock, version, "testapikey"))
		})
	}
}
