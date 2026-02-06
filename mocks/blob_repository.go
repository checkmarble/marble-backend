package mocks

import (
	"context"
	"io"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/stretchr/testify/mock"

	"gocloud.dev/blob"
)

type MockBlobRepository struct {
	mock.Mock
}

func (r *MockBlobRepository) GetBlob(ctx context.Context, bucketUrl, key string, opts ...repositories.GetBlobOption) (models.Blob, error) {
	args := r.Called(ctx, bucketUrl, key, opts)
	return args.Get(0).(models.Blob), args.Error(1)
}

func (r *MockBlobRepository) OpenStream(ctx context.Context, bucketUrl, key string, fileName string) (io.WriteCloser, error) {
	args := r.Called(ctx, bucketUrl, key, fileName)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(io.WriteCloser), args.Error(1)
}

func (r *MockBlobRepository) OpenStreamWithOptions(ctx context.Context, bucketUrl, key string, opts *blob.WriterOptions) (io.WriteCloser, error) {
	args := r.Called(ctx, bucketUrl, key, opts)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(io.WriteCloser), args.Error(1)
}

func (r *MockBlobRepository) RawBucket(ctx context.Context, bucketUrl string) (*blob.Bucket, error) {
	args := r.Called(ctx, bucketUrl)

	return args.Get(0).(*blob.Bucket), args.Error(1)
}

func (r *MockBlobRepository) DeleteFile(ctx context.Context, bucketUrl, key string) error {
	args := r.Called(ctx, bucketUrl, key)
	return args.Error(0)
}

func (r *MockBlobRepository) GenerateSignedUrl(ctx context.Context, bucketUrl, key string) (string, error) {
	args := r.Called(ctx, bucketUrl, key)
	return args.String(0), args.Error(1)
}

func (r *MockBlobRepository) ExtractHost(bucketUrl string) []string {
	args := r.Called(bucketUrl)
	return args.Get(0).([]string)
}
