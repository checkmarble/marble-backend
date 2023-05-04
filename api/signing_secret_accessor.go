package api

import (
	"context"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
)

var privateKeySecretName = "AUTHENTICATION_JWT_SIGNING_KEY"
var publicKeySecretName = "AUTHENTICATION_JWT_VERIFYING_KEY"

type signingSecretAccessorImpl struct {
	privateKey *rsa.PrivateKey
	publicKey  *rsa.PublicKey
}

func readPrivateKey() (*rsa.PrivateKey, error) {
	privateKeyString := os.Getenv("AUTHENTICATION_JWT_SIGNING_KEY")

	block, _ := pem.Decode([]byte(privateKeyString))
	if block == nil || block.Type != "RSA PRIVATE KEY" {
		return nil, fmt.Errorf("failed to decode PEM block containing RSA private key")
	}
	res, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse private key: %v", err)
	}

	return res, nil
}

func readPublicKey() (*rsa.PublicKey, error) {
	publicKeyString := os.Getenv("AUTHENTICATION_JWT_VERIFYING_KEY")

	block, _ := pem.Decode([]byte(publicKeyString))
	if block == nil || block.Type != "PUBLIC KEY" {
		return nil, fmt.Errorf("Failed to decode PEM block containing public key")
	}
	res, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("Failed to parse public key: %v", err)
	}

	return res.(*rsa.PublicKey), nil
}

func (s *signingSecretAccessorImpl) ReadSigningSecrets(ctx context.Context) (*rsa.PrivateKey, *rsa.PublicKey, error) {
	privateKey, err := readPrivateKey()
	if err != nil {
		return nil, nil, fmt.Errorf("Could not read private key %v", err)
	}
	publicKey, err := readPublicKey()
	if err != nil {
		return nil, nil, fmt.Errorf("Could not read public key %v", err)
	}
	s.privateKey = privateKey
	s.publicKey = publicKey

	return s.privateKey, s.publicKey, nil
}
