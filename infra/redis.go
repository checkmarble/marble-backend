package infra

import (
	"github.com/checkmarble/marble-backend/utils"
)

type RedisConfig struct {
	Address       string
	Key           string
	Tls           bool
	TlsSkipVerify bool
}

func InitRedisConfig() (RedisConfig, error) {
	return RedisConfig{
		Address:       utils.GetRequiredEnv[string]("REDIS_HOST"),
		Key:           utils.GetEnv("REDIS_KEY", ""),
		Tls:           utils.GetEnv("REDIS_TLS", false),
		TlsSkipVerify: utils.GetEnv("REDIS_TLS_SKIP_VERIFY", false),
	}, nil
}
