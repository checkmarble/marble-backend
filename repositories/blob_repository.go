package repositories

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
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
	// below: placeholder file url for local testing, url valid for 3 years from 2025/06/02
	placeholderFileUrl = "https://storage.googleapis.com/case-manager-tokyo-country-381508/54624b1f-aac3-4d3c-8fee-75db36436e12/5fd7bbee-b2df-4bc1-9ce8-e218e607c352/d9077dda-7836-45bd-bfde-0c8a3a0c0ad9?Expires=1843502367&GoogleAccessId=local-test-file-signature%40tokyo-country-381508.iam.gserviceaccount.com&Signature=X0FE6jtcJ56p%2B9kNpigxCFyKS5%2FfTVDPUx5Q6qLAQ2Zd1hmQjXEVEjSkLYcCtDGrDfNThkPY9vYCWNPL%2FptVofW5fJ4F1ZVfVUNFVYwhpshiRLBidzLNK7r4Jj%2BwM3wVQTioP4Ms1OUWbENRLdmQ8rPix2n8vyAmzB460oEgdHfva0Q9GrJXCWHQaXgNzj4VZGF7As3nVHQ9ql6n8MUHZNy3y%2BWgLomHZpoFVN5DsfZHIg4HzyWn6z7OgQWkyRm0Nl%2FAXE2gz0UoOnh0j1cblyMinU9KakXjcg5O5p3hswvsGnITm4dzJZEoEkasbnhcH2eTnZwVrB5HGWxzWURtpQ%3D%3D"
)

type BlobRepository interface {
	GetBlob(ctx context.Context, bucketUrl, key string) (models.Blob, error)
	OpenStream(ctx context.Context, bucketUrl, key string, fileName string) (io.WriteCloser, error)
	OpenStreamWithOptions(ctx context.Context, bucketUrl, key string, opts *blob.WriterOptions) (io.WriteCloser, error)
	RawBucket(ctx context.Context, bucketUrl string) (*blob.Bucket, error)
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
	if os.Getenv("DEBUG_BLOB_TRACE") == "true" {
		newCtx, span := tracer.Start(
			ctx,
			"repositories.BlobRepository.openBlobBucket",
			trace.WithAttributes(attribute.String("bucket", bucketUrl)),
		)
		defer span.End()
		ctx = newCtx
	}

	repository.m.Lock()
	defer repository.m.Unlock()

	if repository.buckets[bucketUrl] == nil {
		var bucket *blob.Bucket
		// adapt bucket url with additional values from env variables in the GCS case
		url, err := url.Parse(bucketUrl)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to parse bucket url %s", bucketUrl)
		}
		switch url.Scheme {
		case "gs":
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
		case "s3":
			p := url.Query()
			p.Set("awssdk", "v2")

			// disableSSL became disable_https
			if p.Get("disableSSL") != "" {
				p.Set("disable_https", p.Get("disableSSL"))
				p.Del("disableSSL")
			}
			// s3ForcePathStyle became use_path_style
			// gocloud provides a legacy parameter for the former, but I'd rather we don't rely on it
			if p.Get("s3ForcePathStyle") != "" {
				p.Set("use_path_style", p.Get("s3ForcePathStyle"))
				p.Del("s3ForcePathStyle")
			}

			url.RawQuery = p.Encode()

			bucket, err = blob.OpenBucket(ctx, url.String())
			if err != nil {
				return nil, errors.Wrapf(err, "failed to open bucket %s", bucketUrl)
			}
		default:
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
		"repositories.BlobRepository.GetBlob",
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

func (repository *blobRepository) OpenStreamWithOptions(ctx context.Context, bucketUrl, key string, opts *blob.WriterOptions) (io.WriteCloser, error) {
	bucket, err := repository.openBlobBucket(ctx, bucketUrl)
	if err != nil {
		return nil, err
	}

	return bucket.NewWriter(ctx, key, opts)
}

func (repository *blobRepository) RawBucket(ctx context.Context, bucketUrl string) (*blob.Bucket, error) {
	return repository.openBlobBucket(ctx, bucketUrl)
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

func (repository *blobRepository) GenerateSignedUrl(ctx context.Context, bucketUrl, key string) (string, error) {
	if strings.HasPrefix(bucketUrl, "file://") {
		logger := utils.LoggerFromContext(ctx)
		logger.Warn("Signed URL generation is not supported with a file bucket. Please use a GCS, S3 or Azure bucket instead. Returning a placeholder URL instead.")
		// placeholder file url for local testing, url valid for 3 years from 2025/06/02
		return placeholderFileUrl, nil
	}

	bucket, err := repository.openBlobBucket(ctx, bucketUrl)
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
