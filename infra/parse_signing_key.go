package infra

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"strings"

	"github.com/cockroachdb/errors"

	"github.com/checkmarble/marble-backend/utils"
)

func ParseOrGenerateSigningKey(ctx context.Context, privateKeyString string) *rsa.PrivateKey {
	logger := utils.LoggerFromContext(ctx)
	if privateKeyString == "" {
		logger.InfoContext(ctx, "No AUTHENTICATION_JWT_SIGNING_KEY received, generating a new key pair. This is not recommended for production environments as the key will not be stable in time")
		privateKey, err := generatePrivateKey(2048)
		if err != nil {
			panic("Error generating RSA private key")
		}
		return privateKey
	}

	return parsePrivateKey(privateKeyString)
}

func generatePrivateKey(bitSize int) (*rsa.PrivateKey, error) {
	// Private Key generation
	privateKey, err := rsa.GenerateKey(rand.Reader, bitSize)
	if err != nil {
		return nil, err
	}

	// Validate Private Key
	err = privateKey.Validate()
	if err != nil {
		return nil, err
	}

	return privateKey, nil
}

func parsePrivateKey(privateKeyString string) *rsa.PrivateKey {
	// when a multi-line env variable is passed to the docker container by docker-compose, it escapes the newlines
	privateKeyString = strings.Replace(privateKeyString, "\\n", "\n", -1)
	block, _ := pem.Decode([]byte(privateKeyString))
	if block == nil || block.Type != "RSA PRIVATE KEY" {
		panic(errors.New("failed to decode PEM block containing RSA private key"))
	}

	privateKey, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		panic(errors.New(fmt.Sprintf("Can't load AUTHENTICATION_JWT_SIGNING_KEY private key %s", err)))
	}
	return privateKey
}
