package api

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"log"
	"marble/marble-backend/utils"
)

var privateKeySecretName = "AUTHENTICATION_JWT_SIGNING_KEY"

type SigningSecrets struct {
	privateKey *rsa.PrivateKey
	publicKey  *rsa.PublicKey
}

func readPrivateKey() *rsa.PrivateKey {
	privateKeyString := utils.GetRequiredStringEnv("AUTHENTICATION_JWT_SIGNING_KEY")

	block, _ := pem.Decode([]byte(privateKeyString))
	if block == nil || block.Type != "RSA PRIVATE KEY" {
		log.Fatal("failed to decode PEM block containing RSA private key")
	}
	res, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		log.Fatalf("failed to parse private key: %v", err)
	}

	return res
}

func NewSigningSecrets() SigningSecrets {
	privateKey := readPrivateKey()

	return SigningSecrets{privateKey, &privateKey.PublicKey}
}
