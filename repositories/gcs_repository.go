package repositories

import (
	"context"
	"fmt"
	"marble/marble-backend/models"

	"cloud.google.com/go/storage"
	"golang.org/x/exp/slog"
	"google.golang.org/api/iterator"
)

type GcsRepository interface {
	ListObjects(ctx context.Context, bucketName, prefix string) ([]models.GCSObject, error)
	MoveObject(ctx context.Context, bucketName, source, destination string) error
}

type GcsRepositoryImpl struct {
	// You can create goroutines that concurrently use the same service client to send multiple requests.
	// source: https://aws.github.io/aws-sdk-go-v2/docs/making-requests/
	gcsClient *storage.Client
	logger    *slog.Logger
}

func NewGCSClient() *storage.Client {

	ctx := context.Background()
	client, err := storage.NewClient(ctx)
	if err != nil {
		panic(fmt.Errorf("Failed to load GCS client: %w", err))
	}
	return client
}

func (repository *GcsRepositoryImpl) ListObjects(ctx context.Context, bucketName, prefix string) ([]models.GCSObject, error) {
	bucket := repository.gcsClient.Bucket(bucketName)
	_, err := bucket.Attrs(ctx)
	if err != nil {
		return nil, fmt.Errorf("Failed to get bucket to list GCS objects from bucket %s/%s: %w", bucketName, prefix, err)
	}

	var output []models.GCSObject

	query := &storage.Query{Prefix: ""}
	it := bucket.Objects(ctx, query)
	for {
		attrs, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("Failed to list GCS objects from bucket %s/%s: %v", bucketName, prefix, err)
		}

		r, err := bucket.Object(attrs.Name).NewReader(ctx)
		if err != nil {
			return nil, fmt.Errorf("Failed to read GCS object %s/%s: %v", bucketName, attrs.Name, err)
		}

		output = append(output, models.GCSObject{
			FileName: attrs.Name,
			Reader:   r,
		})
	}

	return output, nil
}

func (repository *GcsRepositoryImpl) MoveObject(ctx context.Context, bucketName, srcName, destName string) error {
	src := repository.gcsClient.Bucket(bucketName).Object(srcName)
	dst := repository.gcsClient.Bucket(bucketName).Object(destName)

	// Optional: set a generation-match precondition to avoid potential race
	// conditions and data corruptions. The request to copy the file is aborted
	// if the object's generation number does not match your precondition.
	// For a dst object that does not yet exist, set the DoesNotExist precondition.
	// Straight from the docs: https://cloud.google.com/storage/docs/copying-renaming-moving-objects?hl=fr#move
	dst = dst.If(storage.Conditions{DoesNotExist: true})

	if _, err := dst.CopierFrom(src).Run(ctx); err != nil {
		return fmt.Errorf("Object(%q).CopierFrom(%q).Run: %w", destName, srcName, err)
	}
	if err := src.Delete(ctx); err != nil {
		return fmt.Errorf("Object(%q).Delete: %w", srcName, err)
	}
	return nil
}
