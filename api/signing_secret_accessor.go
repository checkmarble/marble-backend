package api

import (
	"crypto/rand"
	"crypto/rsa"
	"log"
)

type signingSecretAccessorImpl struct {
	privateKey *rsa.PrivateKey
	publicKey  *rsa.PublicKey
}

func (s *signingSecretAccessorImpl) ReadSigningSecrets() (*rsa.PrivateKey, *rsa.PublicKey, error) {
	// placeholder: right now, it generates a new key pair every time the server is started.
	// Replace this by a read on secret manager
	if s.privateKey == nil {
		privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
		if err != nil {
			log.Fatal(err)
		}
		s.privateKey = privateKey
		s.publicKey = &privateKey.PublicKey
	}

	return s.privateKey, s.publicKey, nil
}

var globalPrivateKey *rsa.PrivateKey
