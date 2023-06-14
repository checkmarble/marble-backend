package repositories

import (
	"context"
	"fmt"
	"io"
	"os"
)

type AwsS3RepositoryFake struct{}

func (repo *AwsS3RepositoryFake) StoreInBucket(ctx context.Context, bucketName string, key string, body io.Reader) error {
	filename := "s3_fake_repo.txt"
	bucket, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("can't open file %s for writing. %w", filename, err)
	}
	defer bucket.Close()

	_, err = io.Copy(bucket, body)
	return err
}
