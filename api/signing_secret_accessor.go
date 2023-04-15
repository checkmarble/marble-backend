package api

import (
	"context"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"hash/crc32"
	"log"
	"os"

	secretmanager "cloud.google.com/go/secretmanager/apiv1"
	"cloud.google.com/go/secretmanager/apiv1/secretmanagerpb"
	"golang.org/x/oauth2/google"
)

var privateKeySecretName = "AUTHENTICATION_JWT_SIGNING_KEY"
var publicKeySecretName = "AUTHENTICATION_JWT_VERIFYING_KEY"

type signingSecretAccessorImpl struct {
	privateKey *rsa.PrivateKey
	publicKey  *rsa.PublicKey
}

func getProjectId() (string, error) {
	// When running locally, best set the environment variable (other option: implement logic to use gloud CLI to get it)
	projectId := os.Getenv("GOOGLE_CLOUD_PROJECT")

	// If running on GCP, the environment variable may not be set automatically
	// In this case, retrieve it from the default credentials (this will not work locally because
	// user-level credentials do not have an associated project: https://github.com/golang/oauth2/issues/241#issuecomment-447902482)
	if projectId == "" {
		log.Println("GOOGLE_CLOUD_PROJECT environment variable is not set, retrieve the value from the default credentials")

		credentials, err := google.FindDefaultCredentials(context.Background())
		if err != nil || credentials.ProjectID == "" {
			log.Printf("Failed to find default credentials: %v", err)
			return "", err
		}
		return credentials.ProjectID, nil
	}
	return projectId, nil
}

func validateChecksum(result *secretmanagerpb.SecretPayload) error {
	crc32c := crc32.MakeTable(crc32.Castagnoli)
	checksum := int64(crc32.Checksum(result.Data, crc32c))
	if checksum != *result.DataCrc32C {
		log.Printf("Data corruption detected.")
		return fmt.Errorf("Data corruption detected.")
	}
	return nil
}

func readKeyPayload(ctx context.Context, client *secretmanager.Client, name string) ([]byte, error) {
	// Build the request.
	req := &secretmanagerpb.AccessSecretVersionRequest{
		Name: name,
	}

	// Call the API.
	result, err := client.AccessSecretVersion(ctx, req)
	if err != nil {
		log.Printf("failed to access secret version: %v", err)
		return nil, err
	}

	err = validateChecksum(result.Payload)
	if err != nil {
		log.Printf("failed to validate checksum: %v", err)
		return nil, err
	}
	return result.Payload.Data, nil
}

func readPrivateKey(ctx context.Context, client *secretmanager.Client, name string) (*rsa.PrivateKey, error) {
	payload, err := readKeyPayload(ctx, client, name)
	if err != nil {
		log.Printf("failed to read key payload: %v", err)
		return nil, err
	}

	block, _ := pem.Decode(payload)
	if block == nil || block.Type != "RSA PRIVATE KEY" {
		log.Fatal("failed to decode PEM block containing RSA private key")
	}
	res, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		log.Printf("failed to parse private key: %v", err)
		return nil, err
	}

	return res, nil
}

func readPublicKey(ctx context.Context, client *secretmanager.Client, name string) (*rsa.PublicKey, error) {
	payload, err := readKeyPayload(ctx, client, name)

	block, _ := pem.Decode(payload)
	if block == nil || block.Type != "PUBLIC KEY" {
		log.Fatal("failed to decode PEM block containing public key")
	}

	res, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		log.Printf("failed to parse public key: %v", err)
		return nil, err
	}

	return res.(*rsa.PublicKey), nil
}

func (s *signingSecretAccessorImpl) ReadSigningSecrets(ctx context.Context) (*rsa.PrivateKey, *rsa.PublicKey, error) {
	projectId, err := getProjectId()
	if err != nil {
		return nil, nil, err
	}
	privateKeySecretFullName := fmt.Sprintf("projects/%s/secrets/%s/versions/latest", projectId, privateKeySecretName)
	publicKeySecretFullName := fmt.Sprintf("projects/%s/secrets/%s/versions/latest", projectId, publicKeySecretName)

	client, err := secretmanager.NewClient(ctx)
	if err != nil {
		log.Printf("Could not create secret manager client %v", err)
		return nil, nil, err
	}
	defer client.Close()

	privateKey, err := readPrivateKey(ctx, client, privateKeySecretFullName)
	if err != nil {
		log.Printf("Could not read private key %v", err)
		return nil, nil, err
	}
	publicKey, err := readPublicKey(ctx, client, publicKeySecretFullName)
	if err != nil {
		log.Printf("Could not read public key %v", err)
		return nil, nil, err
	}
	s.privateKey = privateKey
	s.publicKey = publicKey

	return s.privateKey, s.publicKey, nil
}
