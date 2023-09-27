package repositories

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/checkmarble/marble-backend/models"

	"cloud.google.com/go/storage"
	"google.golang.org/api/iterator"
)

type GcsRepository interface {
	ListFiles(ctx context.Context, bucketName, prefix string) ([]models.GCSFile, error)
	GetFile(ctx context.Context, bucketName, fileName string) (models.GCSFile, error)
	MoveFile(ctx context.Context, bucketName, source, destination string) error
	OpenStream(ctx context.Context, bucketName, fileName string) *storage.Writer
	UpdateFileMetadata(ctx context.Context, bucketName, fileName string, metadata map[string]string) error
}

type GcsRepositoryImpl struct {
	gcsClient *storage.Client
	logger    *slog.Logger
}

func (repository *GcsRepositoryImpl) getGCSClient(ctx context.Context) *storage.Client {
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

func (repository *GcsRepositoryImpl) ListFiles(ctx context.Context, bucketName, prefix string) ([]models.GCSFile, error) {
	bucket := repository.getGCSClient(ctx).Bucket(bucketName)
	_, err := bucket.Attrs(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get bucket to list GCS objects from bucket %s/%s: %w", bucketName, prefix, err)
	}

	var output []models.GCSFile

	query := &storage.Query{Prefix: prefix}
	it := bucket.Objects(ctx, query)
	for {
		attrs, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to list GCS objects from bucket %s/%s: %v", bucketName, prefix, err)
		}

		r, err := bucket.Object(attrs.Name).NewReader(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to read GCS object %s/%s: %v", bucketName, attrs.Name, err)
		}

		output = append(output, models.GCSFile{
			FileName:   attrs.Name,
			Reader:     r,
			BucketName: bucketName,
		})
	}

	return output, nil
}

func (repository *GcsRepositoryImpl) GetFile(ctx context.Context, bucketName, fileName string) (models.GCSFile, error) {
	bucket := repository.getGCSClient(ctx).Bucket(bucketName)
	_, err := bucket.Attrs(ctx)
	if err != nil {
		return models.GCSFile{}, fmt.Errorf("failed to get bucket %s: %w", bucketName, err)
	}
	reader, err := bucket.Object(fileName).NewReader(ctx)
	if err != nil {
		return models.GCSFile{}, fmt.Errorf("failed to read GCS object %s/%s: %v", bucketName, fileName, err)
	}

	return models.GCSFile{
		FileName:   fileName,
		Reader:     reader,
		BucketName: bucketName,
	}, nil
}

func (repository *GcsRepositoryImpl) MoveFile(ctx context.Context, bucketName, srcName, destName string) error {
	gcsClient := repository.getGCSClient(ctx)
	src := gcsClient.Bucket(bucketName).Object(srcName)
	dst := gcsClient.Bucket(bucketName).Object(destName)

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

func (repository *GcsRepositoryImpl) OpenStream(ctx context.Context, bucketName, fileName string) *storage.Writer {
	gcsClient := repository.getGCSClient(ctx)

	writer := gcsClient.Bucket(bucketName).Object(fileName).NewWriter(ctx)
	writer.ChunkSize = 0 // note retries are not supported for chunk size 0.
	return writer
}

func (repository *GcsRepositoryImpl) UpdateFileMetadata(ctx context.Context, bucketName, fileName string, metadata map[string]string) error {
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
