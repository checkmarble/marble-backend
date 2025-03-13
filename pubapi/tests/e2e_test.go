package tests

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"testing"

	"github.com/checkmarble/marble-backend/utils"
)

func TestPublicApi(t *testing.T) {
	for _, version := range []string{"v1"} {
		t.Run(fmt.Sprintf("Public API %s integration tests", version), func(it *testing.T) {
			ctx := context.Background()
			ctx = utils.StoreLoggerInContext(ctx, slog.New(slog.NewTextHandler(io.Discard, nil)))

			pg := setupPostgres(it, ctx)
			sock := setupApi(it, ctx, pg.MustConnectionString(ctx))

			hurl(t, version, sock)
		})
	}
}
