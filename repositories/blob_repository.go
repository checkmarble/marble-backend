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
	"gocloud.dev/blob/gcsblob"
	_ "gocloud.dev/blob/s3blob"
	"gocloud.dev/gcp"

	"cloud.google.com/go/storage"
)

const signedUrlExpiryHours = 1

type BlobRepository interface {
	GetFile(ctx context.Context, bucketName, fileName string) (models.GCSFile, error)
	OpenStream(ctx context.Context, bucketName, fileName string) io.WriteCloser
	DeleteFile(ctx context.Context, bucketName, fileName string) error
	UpdateFileMetadata(ctx context.Context, bucketName, fileName string, metadata map[string]string) error
	GenerateSignedUrl(ctx context.Context, bucketName, fileName string) (string, error)
}

type blobRepository struct {
	gcsClient                    *storage.Client
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

func (repository *blobRepository) getGCSClient(ctx context.Context) *storage.Client {
	// Lazy load the GCS client, as it is used only in one batch usecase, to avoid requiring GCS credentials for all devs
	if repository.gcsClient != nil {
		return repository.gcsClient
	}

	client, err := storage.NewClient(ctx)
	if err != nil {
		panic(fmt.Errorf("failed to load GCS client: %w", err))
	}
	repository.gcsClient = client
	return client
}

func (repository *blobRepository) openBlobBucket(ctx context.Context, bucketUrl string) (*blob.Bucket, error) {
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
			// // in the GCS case, we need to set those values in the url
			// query := url.Query()
			// if accessId := query.Get("access_id"); accessId == "" {
			// 	query.Set("access_id", repository.googleAccessId)
			// }
			// if keyPath := query.Get("private_key_path"); keyPath == "" {
			// 	// query.Set("private_key_path", repository.googleApplicationCredentials)
			// 	query.Set("private_key_path", ".service_account_key/key.pem")
			// }
			// url.RawQuery = query.Encode()
			// bucketUrl = url.String()

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
		repository.buckets[bucketUrl] = bucket
	}
	return repository.buckets[bucketUrl], nil
}

func (repository *blobRepository) GetFile(ctx context.Context, bucketName, fileName string) (models.GCSFile, error) {
	tracer := utils.OpenTelemetryTracerFromContext(ctx)
	ctx, span := tracer.Start(
		ctx,
		"repositories.GcsRepository.GetFile",
		trace.WithAttributes(attribute.String("bucket", bucketName)),
		trace.WithAttributes(attribute.String("fileName", fileName)),
	)
	defer span.End()
	bucket := repository.getGCSClient(ctx).Bucket(bucketName)

	ctxBucket, span2 := tracer.Start(
		ctx,
		"repositories.GcsRepository.GetFile - bucket attrs",
	)
	defer span2.End()
	_, err := bucket.Attrs(ctxBucket)
	if err != nil {
		return models.GCSFile{}, fmt.Errorf("failed to get bucket %s: %w", bucketName, err)
	}
	span2.End()

	ctx, span = tracer.Start(
		ctx,
		"repositories.GcsRepository.GetFile - file reader",
	)
	defer span.End()
	reader, err := bucket.Object(fileName).NewReader(ctx)
	if err != nil {
		return models.GCSFile{}, fmt.Errorf("failed to read GCS object %s/%s: %w", bucketName, fileName, err)
	}

	return models.GCSFile{
		FileName:   fileName,
		Reader:     reader,
		BucketName: bucketName,
	}, nil
}

func (repository *blobRepository) OpenStream(ctx context.Context, bucketName, fileName string) io.WriteCloser {
	gcsClient := repository.getGCSClient(ctx)

	writer := gcsClient.Bucket(bucketName).Object(fileName).NewWriter(ctx)
	writer.ChunkSize = 0 // note retries are not supported for chunk size 0.
	return writer
}

func (repository *blobRepository) UpdateFileMetadata(ctx context.Context,
	bucketName, fileName string, metadata map[string]string,
) error {
	gcsClient := repository.getGCSClient(ctx)
	defer gcsClient.Close()

	ctx, cancel := context.WithTimeout(ctx, time.Second*10)
	defer cancel()

	object := gcsClient.Bucket(bucketName).Object(fileName)

	// Optional: set a metageneration-match precondition to avoid potential race
	// conditions and data corruptions. The request to update is aborted if the
	// object's metageneration does not match your precondition.
	attrs, err := object.Attrs(ctx)
	if err != nil {
		return fmt.Errorf("object.Attrs: %w", err)
	}
	object = object.If(storage.Conditions{MetagenerationMatch: attrs.Metageneration})

	objectAttrsToUpdate := storage.ObjectAttrsToUpdate{Metadata: metadata}

	if _, err := object.Update(ctx, objectAttrsToUpdate); err != nil {
		return fmt.Errorf("ObjectHandle(%q).Update: %w", fileName, err)
	}

	return nil
}

func (repository *blobRepository) DeleteFile(ctx context.Context, bucketName, fileName string) error {
	gcsClient := repository.getGCSClient(ctx)
	defer gcsClient.Close()

	ctx, cancel := context.WithTimeout(ctx, time.Second*10)
	defer cancel()

	object := gcsClient.Bucket(bucketName).Object(fileName)

	if err := object.Delete(ctx); err != nil {
		return errors.Wrap(err, fmt.Sprintf("Error deleting file: %s", fileName))
	}

	return nil
}

func (repo *blobRepository) GenerateSignedUrl_(ctx context.Context, bucketName, fileName string) (string, error) {
	// This code will typically not run locally if you target the real GCS repository, because SignedURL only works with service account credentials (not end user credentials)
	// Hence, run the code locally with the fake GCS repository always
	// bucketName = "case-manager-tokyo-country-381508"
	bucket := repo.getGCSClient(ctx).Bucket(bucketName)

	return bucket.
		SignedURL(
			fileName,
			&storage.SignedURLOptions{
				Method:         http.MethodGet,
				Expires:        time.Now().Add(signedUrlExpiryHours * time.Hour),
				PrivateKey:     repo.serviceAccountPemKey,
				GoogleAccessID: repo.googleAccessId,
			},
		)
}

func (repo *blobRepository) GenerateSignedUrl(ctx context.Context, bucketName, fileName string) (string, error) {
	// This code will typically not run locally if you target the real GCS repository, because SignedURL only works with service account credentials (not end user credentials)
	// Hence, run the code locally with the fake GCS repository always
	bucket, err := repo.openBlobBucket(ctx, bucketName)
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
