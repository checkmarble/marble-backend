package repositories

import (
	"context"
	"fmt"
	"io"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/pkg/errors"
)

type AwsS3Repository struct {
	// You can create goroutines that concurrently use the same service client to send multiple requests.
	// source: https://aws.github.io/aws-sdk-go-v2/docs/making-requests/
	s3Client *s3.Client
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

func (repo *AwsS3Repository) StoreInBucket(ctx context.Context, bucketName string, key string, body io.Reader) error {
	logger := utils.LoggerFromContext(ctx)
	uploader := manager.NewUploader(repo.s3Client)

	location, err := uploader.Upload(ctx, &s3.PutObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(key),
		Body:   body,
	})
	if err != nil {
		return errors.Errorf("Couldn't upload fileName to %v:%v. Here's why: %v\n", bucketName, key, err)
	}

	logger.InfoContext(ctx, fmt.Sprintf("Successfully uploaded to s3 to %v", location.Location))
	return nil
}
