package repositories

import (
	"bytes"
	"context"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"

	"github.com/cockroachdb/errors"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"gocloud.dev/blob"
	_ "gocloud.dev/blob/azureblob"
	_ "gocloud.dev/blob/fileblob"
	"gocloud.dev/blob/gcsblob"
	_ "gocloud.dev/blob/s3blob"
	"gocloud.dev/gcp"
)

const signedUrlExpiryHours = 1

type BlobRepository interface {
	GetBlob(ctx context.Context, bucketUrl, fileName string) (models.Blob, error)
	OpenStream(ctx context.Context, bucketUrl, fileName string) (io.WriteCloser, error)
	DeleteFile(ctx context.Context, bucketUrl, fileName string) error
	GenerateSignedUrl(ctx context.Context, bucketUrl, fileName string) (string, error)
}

type blobRepository struct {
	buckets                      map[string]*blob.Bucket
	m                            sync.Mutex
	googleAccessId               string
	googleApplicationCredentials string
	serviceAccountPemKey         []byte
}

func NewBlobRepository(googleApplicationCredentials string) BlobRepository {
	var pemKey []byte
	var googleAccessId string
	if googleApplicationCredentials != "" {
		key, err := os.ReadFile(googleApplicationCredentials)
		if err != nil {
			panic(errors.Wrap(err, "failed to read service account key"))
		}
		pemKey, err = gcpServiceAccountKeyToPEM(key)
		if err != nil {
			panic(errors.Wrap(err, "failed to convert service account key to PEM"))
		}

		googleAccessId, err = gcpServiceAccountKeyToGoogleAccessId(key)
		if err != nil {
			panic(errors.Wrap(err, "failed to get google access id"))
		}
	}

	return &blobRepository{
		buckets:                      make(map[string]*blob.Bucket),
		googleAccessId:               googleAccessId,
		googleApplicationCredentials: googleApplicationCredentials,
		serviceAccountPemKey:         pemKey,
	}
}

func (repository *blobRepository) openBlobBucket(ctx context.Context, bucketUrl string) (*blob.Bucket, error) {
	tracer := utils.OpenTelemetryTracerFromContext(ctx)
	ctx, span := tracer.Start(
		ctx,
		"repositories.BlobRepository.openBlobBucket",
		trace.WithAttributes(attribute.String("bucket", bucketUrl)),
	)
	defer span.End()

	if repository.buckets[bucketUrl] == nil {
		repository.m.Lock()
		defer repository.m.Unlock()

		var bucket *blob.Bucket
		// adapt bucket url with additional values from env variables in the GCS case
		url, err := url.Parse(bucketUrl)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to parse bucket url %s", bucketUrl)
		}
		if url.Scheme == "gs" {
			// in the GCS case, we need to set those values in the url
			creds, err := gcp.DefaultCredentials(ctx)
			if err != nil {
				return nil, err
			}
			client, err := gcp.NewHTTPClient(
				gcp.DefaultTransport(),
				gcp.CredentialsTokenSource(creds))
			if err != nil {
				return nil, err
			}

			bucket, err = gcsblob.OpenBucket(ctx, client, url.Host, &gcsblob.Options{
				GoogleAccessID: repository.googleAccessId,
				PrivateKey:     repository.serviceAccountPemKey,
			})
			if err != nil {
				return nil, err
			}
		} else {
			bucket, err = blob.OpenBucket(ctx, bucketUrl)
			if err != nil {
				return nil, errors.Wrapf(err, "failed to open bucket %s", bucketUrl)
			}
		}

		ok, err := bucket.IsAccessible(ctx)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to check bucket accessibility %s", bucketUrl)
		} else if !ok {
			return nil, errors.Newf("bucket %s is not accessible", bucketUrl)
		}

		repository.buckets[bucketUrl] = bucket
	}
	return repository.buckets[bucketUrl], nil
}

func (repository *blobRepository) GetBlob(ctx context.Context, bucketUrl, fileName string) (models.Blob, error) {
	tracer := utils.OpenTelemetryTracerFromContext(ctx)
	ctx, span := tracer.Start(
		ctx,
		"repositories.BlobRepository.openBlobBucket",
		trace.WithAttributes(attribute.String("bucket", bucketUrl)),
		trace.WithAttributes(attribute.String("fileName", fileName)),
	)
	defer span.End()
	bucket, err := repository.openBlobBucket(ctx, bucketUrl)
	if err != nil {
		return models.Blob{}, err
	}

	ctx, span = tracer.Start(
		ctx,
		"repositories.BlobRepository.GetFile - file reader",
	)
	defer span.End()

	ok, err := bucket.Exists(ctx, fileName)
	if err != nil {
		return models.Blob{}, errors.Wrapf(err, "failed to check if file %s exists in bucket %s", fileName, bucketUrl)
	} else if !ok {
		return models.Blob{}, errors.Wrapf(
			models.NotFoundError,
			"file %s does not exist in bucket %s", fileName, bucketUrl,
		)
	}

	reader, err := bucket.NewReader(ctx, fileName, nil)
	if err != nil {
		return models.Blob{}, errors.Wrapf(err, "failed to read GCS object %s/%s", bucketUrl, fileName)
	}

	return models.Blob{FileName: fileName, ReadCloser: reader}, nil
}

func (repository *blobRepository) OpenStream(ctx context.Context, bucketUrl, fileName string) (io.WriteCloser, error) {
	bucket, err := repository.openBlobBucket(ctx, bucketUrl)
	if err != nil {
		return nil, err
	}

	return bucket.NewWriter(ctx, fileName, &blob.WriterOptions{
		ContentDisposition: fmt.Sprintf("attachment; filename=\"%s\"", fileName),
	})
}

func (repository *blobRepository) DeleteFile(ctx context.Context, bucketUrl, fileName string) error {
	bucket, err := repository.openBlobBucket(ctx, bucketUrl)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(ctx, time.Second*10)
	defer cancel()

	return bucket.Delete(ctx, fileName)
}

func (repo *blobRepository) GenerateSignedUrl(ctx context.Context, bucketUrl, fileName string) (string, error) {
	// This code will typically not run locally if you target the real GCS repository, because SignedURL only works with service account credentials (not end user credentials)
	// Hence, run the code locally with the fake GCS repository always
	bucket, err := repo.openBlobBucket(ctx, bucketUrl)
	if err != nil {
		return "", err
	}

	return bucket.SignedURL(
		ctx,
		fileName,
		&blob.SignedURLOptions{
			Method: http.MethodGet,
			Expiry: signedUrlExpiryHours * time.Hour,
		})
}

func gcpServiceAccountKeyToPEM(key []byte) ([]byte, error) {
	type serviceAccountKey struct {
		PrivateKey     string `json:"private_key"`
		GoogleAccessId string `json:"client_email"` //nolint:tagliatelle
	}
	// Parse the JSON data from the service account key file
	var k serviceAccountKey
	err := json.Unmarshal(key, &k)
	if err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal service account key")
	}

	block, _ := pem.Decode([]byte(k.PrivateKey))
	if block == nil {
		return nil, errors.Wrap(err, "Failed to decode PEM")
	}

	buf := new(bytes.Buffer)
	err = pem.Encode(buf, block)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to encode PEM")
	}

	err = os.WriteFile(".service_account_key/key.pem", buf.Bytes(), 0o644)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to write PEM to file")
	}

	return buf.Bytes(), nil
}

func gcpServiceAccountKeyToGoogleAccessId(key []byte) (string, error) {
	// Parse the JSON data from the service account key file

	var sa struct {
		ClientEmail        string `json:"client_email"`
		SAImpersonationURL string `json:"service_account_impersonation_url"` //nolint:tagliatelle
		CredType           string `json:"type"`                              //nolint:tagliatelle
	}

	err := json.Unmarshal(key, &sa)
	if err != nil {
		return "", errors.Wrap(err, "failed to unmarshal service account key")
	}

	switch sa.CredType {
	case "impersonated_service_account", "external_account":
		start, end := strings.LastIndex(sa.SAImpersonationURL, "/"),
			strings.LastIndex(sa.SAImpersonationURL, ":")

		if end <= start {
			return "", errors.New("error parsing external or impersonated service account credentials")
		} else {
			return sa.SAImpersonationURL[start+1 : end], nil
		}
	case "service_account":
		if sa.ClientEmail != "" {
			return sa.ClientEmail, nil
		}
		return "", errors.New("empty service account client email")
	default:
		return "", errors.New("unable to parse credentials; only service_account, external_account and impersonated_service_account credentials are supported")
	}
}
