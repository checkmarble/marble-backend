package repositories

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/checkmarble/marble-backend/infra"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"
	"golang.org/x/oauth2/google"

	credentials "cloud.google.com/go/iam/credentials/apiv1"
	"cloud.google.com/go/iam/credentials/apiv1/credentialspb"
	"github.com/cockroachdb/errors"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"gocloud.dev/blob"
	_ "gocloud.dev/blob/azureblob"
	_ "gocloud.dev/blob/fileblob"
	"gocloud.dev/blob/gcsblob"
	_ "gocloud.dev/blob/s3blob"
	"gocloud.dev/gcp"
	"google.golang.org/api/compute/v1"
)

const (
	signedUrlExpiry = 1 * time.Hour
	// below: placeholder file url for local testing, url valid for 3 years from 2024/09/16
	placeholderFileUrl = "https://storage.googleapis.com/case-manager-tokyo-country-381508/54624b1f-aac3-4d3c-8fee-75db36436e12/5fd7bbee-b2df-4bc1-9ce8-e218e607c352/d9077dda-7836-45bd-bfde-0c8a3a0c0ad9?Expires=1821107332&GoogleAccessId=marble-backend-cloud-run%40tokyo-country-381508.iam.gserviceaccount.com&Signature=YYGF0msoL%2FmiIb3BKroFDRP0DzZrHQF3pS5VudT0OymeNnmxoIZS5DOycPaCcRa%2FMbRh454YEpAQGT%2F6Xf5dGWo%2FEj7UfmoKmPPyRGZ82qo9lr1ZMdvveBtBmSdzepgk6EBkWxWX3Ov0ZOguD58pKVy4Q0WzaMl5aD8dN8jv2ExuCfGRvNCpfvP43eONEtox6ilPkVkq4Bqhq9BHo4OBQj%2FuU8BLfnge35Db3IIy2f69CJ0wagLPiYkWfu5GODgoXMsjL0JtNEryeCJMH2ocXrTlV0XD00bx%2F8vhaHCHY9o%2Ft1V0sHmd7CoIa3bUsx4gixCuxivqvc5xLeDkgeTf%2FA%3D%3D"
)

type BlobRepository interface {
	GetBlob(ctx context.Context, bucketUrl, key string) (models.Blob, error)
	OpenStream(ctx context.Context, bucketUrl, key string, fileName string) (io.WriteCloser, error)
	DeleteFile(ctx context.Context, bucketUrl, key string) error
	GenerateSignedUrl(ctx context.Context, bucketUrl, key string) (string, error)
}

type blobRepository struct {
	buckets   map[string]*blob.Bucket
	m         sync.Mutex
	gcpConfig infra.GcpConfig
}

func NewBlobRepository(gcpConfig infra.GcpConfig) BlobRepository {
	return &blobRepository{
		buckets:   make(map[string]*blob.Bucket),
		gcpConfig: gcpConfig,
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
			creds, err := google.FindDefaultCredentials(ctx, compute.CloudPlatformScope)
			if err != nil {
				return nil, err
			}
			client, err := gcp.NewHTTPClient(gcp.DefaultTransport(), gcp.CredentialsTokenSource(creds))
			if err != nil {
				return nil, err
			}

			bucket, err = gcsblob.OpenBucket(ctx, client, url.Host, &gcsblob.Options{
				GoogleAccessID: repository.gcpConfig.PrincipalEmail,
				MakeSignBytes: func(requestCtx context.Context) gcsblob.SignBytesFunc {
					return func(p []byte) ([]byte, error) {
						signClient, err := credentials.NewIamCredentialsClient(ctx)
						if err != nil {
							return nil, err
						}

						resp, err := signClient.SignBlob(requestCtx, &credentialspb.SignBlobRequest{
							Name:    repository.gcpConfig.PrincipalEmail,
							Payload: p,
						})
						if err != nil {
							return nil, err
						}

						return resp.GetSignedBlob(), nil
					}
				},
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

func (repository *blobRepository) GetBlob(ctx context.Context, bucketUrl, key string) (models.Blob, error) {
	tracer := utils.OpenTelemetryTracerFromContext(ctx)
	ctx, span := tracer.Start(
		ctx,
		"repositories.BlobRepository.openBlobBucket",
		trace.WithAttributes(attribute.String("bucket", bucketUrl)),
		trace.WithAttributes(attribute.String("key", key)),
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

	ok, err := bucket.Exists(ctx, key)
	if err != nil {
		return models.Blob{}, errors.Wrapf(err, "failed to check if file %s exists in bucket %s", key, bucketUrl)
	} else if !ok {
		return models.Blob{}, errors.Wrapf(
			models.NotFoundError,
			"file %s does not exist in bucket %s", key, bucketUrl,
		)
	}

	reader, err := bucket.NewReader(ctx, key, nil)
	if err != nil {
		return models.Blob{}, errors.Wrapf(err, "failed to read blob %s/%s", bucketUrl, key)
	}

	return models.Blob{FileName: key, ReadCloser: reader}, nil
}

func (repository *blobRepository) OpenStream(ctx context.Context, bucketUrl, key string, fileName string) (io.WriteCloser, error) {
	bucket, err := repository.openBlobBucket(ctx, bucketUrl)
	if err != nil {
		return nil, err
	}

	return bucket.NewWriter(ctx, key, &blob.WriterOptions{
		ContentDisposition: fmt.Sprintf("attachment; filename=\"%s\"", fileName),
	})
}

func (repository *blobRepository) DeleteFile(ctx context.Context, bucketUrl, key string) error {
	bucket, err := repository.openBlobBucket(ctx, bucketUrl)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(ctx, time.Second*10)
	defer cancel()

	return bucket.Delete(ctx, key)
}

func (repo *blobRepository) GenerateSignedUrl(ctx context.Context, bucketUrl, key string) (string, error) {
	if strings.HasPrefix(bucketUrl, "file://") {
		logger := utils.LoggerFromContext(ctx)
		logger.Warn("Signed URL generation is not supported with a file bucket. Please use a GCS, S3 or Azure bucket instead. Returning a placeholder URL instead.")
		// placeholder file, url valid for 3 years from 2024/09/16
		return placeholderFileUrl, nil
	}

	bucket, err := repo.openBlobBucket(ctx, bucketUrl)
	if err != nil {
		return "", err
	}

	return bucket.SignedURL(
		ctx,
		key,
		&blob.SignedURLOptions{
			Method: http.MethodGet,
			Expiry: signedUrlExpiry,
		})
}
