// Go 1.24 removed the ability to generate RSA key < 1024 bits, we re-enabled it for tests since we do not use the key.
//go:debug rsa1024min=0

package tests

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"database/sql"
	"fmt"
	"io"
	"log"
	"log/slog"
	"net"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/checkmarble/marble-backend/api"
	"github.com/checkmarble/marble-backend/infra"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases"
	"github.com/gavv/httpexpect/v2"
	"github.com/gin-gonic/gin"
	"github.com/go-testfixtures/testfixtures/v3"
	"github.com/pressly/goose/v3"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

func client(t *testing.T, sock, version, apiKey string) *httpexpect.Expect {
	httpc := http.Client{
		Transport: &http.Transport{
			DialContext: func(_ context.Context, _, _ string) (net.Conn, error) {
				return net.Dial("unix", sock)
			},
		},
	}

	e := httpexpect.WithConfig(httpexpect.Config{
		TestName: t.Name(),
		Client:   &httpc,
		BaseURL:  fmt.Sprintf("http://localhost/%s", version),
		Reporter: httpexpect.NewAssertReporter(t),
		Printers: []httpexpect.Printer{
			httpexpect.NewDebugPrinter(t, true),
		},
	})

	return e.Builder(func(r *httpexpect.Request) {
		r.WithHeader("x-api-key", apiKey)
	})
}

func setupPostgres(t *testing.T, ctx context.Context) *postgres.PostgresContainer {
	t.Helper()

	testcontainers.Logger = log.New(io.Discard, "", 0)
	goose.SetLogger(goose.NopLogger())

	pg, err := postgres.Run(
		ctx,
		"postgres:15",
		postgres.WithDatabase("marble_test"),
		postgres.WithUsername("postgres"),
		postgres.WithPassword("marble"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(5*time.Second)),
	)

	t.Cleanup(func() {
		if err := testcontainers.TerminateContainer(pg); err != nil {
			t.Fatal(err)
		}
	})

	if err != nil {
		t.Fatal(err)
	}

	dsn := pg.MustConnectionString(ctx)
	pgConfig := infra.PgConfig{ConnectionString: dsn}
	migrator := repositories.NewMigrater(pgConfig)

	if err = migrator.Run(ctx); err != nil {
		log.Fatal(err)
	}

	conn, err := sql.Open("pgx", dsn)
	if err != nil {
		log.Fatal(err)
	}

	fixtures, err := testfixtures.New(
		testfixtures.Database(conn),
		testfixtures.Dialect("postgres"),
		testfixtures.FilesMultiTables("fixtures/base/base.yml"),
		testfixtures.Directory("fixtures"),
	)
	if err != nil {
		t.Fatal(err)
	}

	if err := fixtures.Load(); err != nil {
		t.Fatal(err)
	}

	return pg
}

func setupApi(t *testing.T, ctx context.Context, dsn string) string {
	t.Helper()

	gin.SetMode(gin.ReleaseMode)

	pool, err := infra.NewPostgresConnectionPool(ctx, dsn, nil, 10)
	if err != nil {
		log.Fatalf("Could not create connection pool: %s", err)
	}

	cfg := api.Configuration{Env: "development", MarbleAppUrl: "http://x", DefaultTimeout: 5 * time.Second}
	key, err := rsa.GenerateKey(rand.Reader, 128)
	if err != nil {
		t.Fatal(err)
	}

	deps := api.InitDependencies(ctx, cfg, pool, key, nil)
	openSanctions := infra.InitializeOpenSanctions(http.DefaultClient, " ", " ", " ")
	repos := repositories.NewRepositories(pool, "",
		repositories.WithOpenSanctions(openSanctions))
	uc := usecases.NewUsecases(repos, usecases.WithLicense(models.NewFullLicense()), usecases.WithOpensanctions(true))
	router := api.InitRouterMiddlewares(ctx, cfg, nil, infra.TelemetryRessources{})

	server := api.NewServer(
		router,
		cfg,
		uc,
		deps.Authentication,
		deps.TokenHandler,
		slog.Default(),
		api.WithLocalTest(true),
	)

	srv := http.Server{Handler: server.Handler}

	sockDir, err := os.MkdirTemp(os.TempDir(), "marble")
	if err != nil {
		t.Fatal(err)
	}
	sockFile := fmt.Sprintf("%s/api.sock", sockDir)

	t.Cleanup(func() {
		_ = srv.Close()
		_ = server.Shutdown(ctx)
		_ = os.RemoveAll(sockDir)
	})

	listener, err := net.Listen("unix", sockFile)
	if err != nil {
		t.Fatal(err)
	}

	go func() {
		_ = srv.Serve(listener)
	}()

	return sockFile
}
