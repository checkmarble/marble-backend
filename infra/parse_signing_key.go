package infra

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"log"
	"strings"
)

func MustParseSigningKey(privateKeyString string) *rsa.PrivateKey {
	// when a multi-line env variable is passed to the docker container by docker-compose, it escapes the newlines
	privateKeyString = strings.Replace(privateKeyString, "\\n", "\n", -1)
	block, _ := pem.Decode([]byte(privateKeyString))
	if block == nil || block.Type != "RSA PRIVATE KEY" {
		log.Fatalf("failed to decode PEM block containing RSA private key")
	}

	privateKey, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		log.Fatalf("Can't load AUTHENTICATION_JWT_SIGNING_KEY private key %s", err)
	}
	return privateKey
}
