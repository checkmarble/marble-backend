package repositories

import (
	"context"
	"fmt"
	"io"
	"log/slog"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	"github.com/pkg/errors"
)

type AwsS3Repository interface {
	StoreInBucket(ctx context.Context, bucketName string, key string, body io.Reader) error
}

type AwsS3RepositoryImpl struct {
	// You can create goroutines that concurrently use the same service client to send multiple requests.
	// source: https://aws.github.io/aws-sdk-go-v2/docs/making-requests/
	s3Client *s3.Client
	logger   *slog.Logger
}

func NewS3Client() *s3.Client {

	// aws auto configure itself with the following environment variables:
	// AWS_REGION, AWS_ACCESS_KEY, AWS_SECRET_KEY
	conf, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		panic(fmt.Errorf("fail to load AWS config: %w", err))
	}

	return s3.NewFromConfig(conf)
}

func (repo *AwsS3RepositoryImpl) StoreInBucket(ctx context.Context, bucketName string, key string, body io.Reader) error {

	uploader := manager.NewUploader(repo.s3Client)

	location, err := uploader.Upload(ctx, &s3.PutObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(key),
		Body:   body,
	})
	if err != nil {
		return errors.Errorf("Couldn't upload fileName to %v:%v. Here's why: %v\n", bucketName, key, err)
	}

	repo.logger.Info(fmt.Sprintf("Successfully uploaded to s3 to %v", location.Location))
	return nil
}
