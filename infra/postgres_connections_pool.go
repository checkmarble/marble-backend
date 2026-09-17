package infra

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"os"
	"strings"
	"time"

	"cloud.google.com/go/cloudsqlconn"
	"github.com/avast/retry-go/v4"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/exaring/otelpgx"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	pgxgeom "github.com/twpayne/pgx-geom"
	"go.opentelemetry.io/otel/trace"
)

const (
	DEFAULT_MAX_CONNECTIONS  = 40
	MAX_CONNECTION_IDLE_TIME = 5 * time.Minute
)

type ClientDbConfig struct {
	ConnectionString       string `json:"connection_string"`
	CloudSqlConnectionName string `json:"cloudsql_connection_name"` //nolint:tagliatelle
	MaxConns               int    `json:"max_conns"`
	SchemaName             string `json:"schema_name"`
	ImpersonateRole        string `json:"impersonate_role"`
}

type CloudSqlLogger struct {
	*slog.Logger
}

var CLOUD_SQL_OMIT_MESSAGES = []string{
	"dial succesful",
	"i/o timeout",
}

func (l CloudSqlLogger) Debugf(ctx context.Context, format string, args ...any) {
	msg := fmt.Sprintf(format, args...)

	for _, banned := range CLOUD_SQL_OMIT_MESSAGES {
		if strings.Contains(msg, banned) {
			return
		}
	}

	l.InfoContext(ctx, msg)
}

func NewPostgresConnectionPool(
	ctx context.Context,
	appName string,
	connectionString string,
	tp trace.TracerProvider,
	maxConnections int,
	impersonateRole string,
	cloudsqlConnectionName string,
) (*pgxpool.Pool, *cloudsqlconn.Dialer, error) {
	logger := CloudSqlLogger{utils.LoggerFromContext(ctx)}

	cfg, err := pgxpool.ParseConfig(connectionString)
	if err != nil {
		return nil, nil, fmt.Errorf("create connection pool: %w", err)
	}

	var poolDialer *cloudsqlconn.Dialer

	if cloudsqlConnectionName != "" {
		// Cloud SQL proxy does not support Postgres-native TLS, since the whole TCP connection is
		// wrapped in a mTLS tunnel.
		cfg.ConnConfig.TLSConfig = nil

		opts := []cloudsqlconn.Option{
			cloudsqlconn.WithContextDebugLogger(logger),
		}

		if cfg.ConnConfig.Password == "" {
			opts = append(opts, cloudsqlconn.WithIAMAuthN())
		}

		dialer, err := cloudsqlconn.NewDialer(context.Background(), opts...)
		if err != nil {
			return nil, nil, fmt.Errorf("create cloudsql dialer: %w", err)
		}

		cfg.ConnConfig.DialFunc = func(ctx context.Context, network, addr string) (net.Conn, error) {
			return dialer.Dial(ctx, cloudsqlConnectionName)
		}

		poolDialer = dialer
	}

	ops := []otelpgx.Option{}
	if tp != nil {
		ops = append(ops, otelpgx.WithTracerProvider(tp))
	}
	cfg.ConnConfig.Tracer = otelpgx.NewTracer(ops...)
	cfg.MaxConns = int32(maxConnections)
	if cfg.MaxConns == 0 {
		cfg.MaxConns = DEFAULT_MAX_CONNECTIONS
	}
	cfg.MaxConnIdleTime = MAX_CONNECTION_IDLE_TIME
	cfg.MaxConnLifetimeJitter = 5 * time.Minute

	cfg.ConnConfig.RuntimeParams = map[string]string{
		"application_name": appName,
	}

	cfg.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
		if impersonateRole != "" {
			if _, err := conn.Exec(ctx, "SET ROLE "+pgx.Identifier([]string{impersonateRole}).Sanitize()); err != nil {
				return err
			}
		}

		if err := pgxgeom.Register(ctx, conn); err != nil {
			return err
		}

		return nil
	}

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		if poolDialer != nil {
			poolDialer.Close()
		}

		return nil, nil, fmt.Errorf("unable to create connection pool: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	err = retry.Do(
		func() error {
			if err := pool.Ping(ctx); err != nil {
				return fmt.Errorf("NewPostgresConnectionPool.Ping error: %w", err)
			}
			return err
		},
		retry.Attempts(3),
		retry.LastErrorOnly(true),
	)

	if err != nil {
		pool.Close()
		if poolDialer != nil {
			poolDialer.Close()
		}

		return nil, nil, err
	}

	return pool, poolDialer, nil
}

func ParseClientDbConfig(filename string) (map[string]ClientDbConfig, error) {
	if filename == "" {
		return nil, nil
	}
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	clientDbConfigs := make(map[string]ClientDbConfig)
	if err := json.NewDecoder(file).Decode(&clientDbConfigs); err != nil {
		return nil, err
	}
	return clientDbConfigs, nil
}
