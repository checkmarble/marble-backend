package infra

import (
	"crypto/x509"

	"github.com/checkmarble/marble-backend/utils"
	"github.com/cockroachdb/errors"
)

type RedisConfig struct {
	Address  string
	Username string
	Key      string
	CaCerts  *x509.CertPool
}

func InitRedisConfig() (RedisConfig, error) {
	var certPool *x509.CertPool

	if certs := utils.GetEnv("REDIS_CACERTS", ""); certs != "" {
		certPool = x509.NewCertPool()

		if !certPool.AppendCertsFromPEM([]byte(certs)) {
			return RedisConfig{}, errors.New("no certificate could be read for redis tls chain")
		}
	}

	return RedisConfig{
		Address: utils.GetRequiredEnv[string]("REDIS_HOST"),
		Key:     utils.GetEnv("REDIS_KEY", ""),
		CaCerts: certPool,
	}, nil
}
