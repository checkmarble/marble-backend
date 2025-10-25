package repositories

import (
	"context"
	"crypto/tls"
	"time"

	"github.com/checkmarble/marble-backend/infra"
	"github.com/cockroachdb/errors"
	"github.com/redis/go-redis/v9"
)

const (
	RedisGcpRefreshInterval = 45 * time.Minute
)

type RedisClient struct {
	client *redis.Client
}

func NewRedisClient(cfg infra.RedisConfig) (*RedisClient, error) {
	ctx := context.Background()

	var tlsConfig *tls.Config

	// TODO: decide if we want certificate validation (which means bringing
	// along a PEM in the configuration) or if we are okay to ignore TLS
	// authentication.
	if cfg.CaCerts != nil {
		tlsConfig = &tls.Config{
			MinVersion: tls.VersionTLS12,
			RootCAs:    cfg.CaCerts,
		}
	}

	client := &RedisClient{
		client: redis.NewClient(&redis.Options{
			Addr:      cfg.Address,
			Username:  "",
			Password:  cfg.Key,
			TLSConfig: tlsConfig,
		}),
	}

	if err := client.client.Ping(ctx).Err(); err != nil {
		return nil, errors.Wrap(err, "could not check redis connectivity")
	}

	return client, nil
}
