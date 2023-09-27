package repositories

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/checkmarble/marble-backend/models"
)

type GcsRepositoryFake struct{}

const tempFilesDirectory = "tempFiles"

func (repo *GcsRepositoryFake) ListFiles(ctx context.Context, bucketName, prefix string) ([]models.GCSFile, error) {
	cwd, _ := os.Getwd()
	files, err := os.ReadDir(filepath.Join(cwd, tempFilesDirectory))
	if err != nil {
		return nil, err
	}

	var gcsFiles []models.GCSFile
	for _, file := range files {
		fileReader, err := os.Open(filepath.Join(cwd, tempFilesDirectory, file.Name()))
		if err != nil {
			return []models.GCSFile{}, err
		}
		gcsFiles = append(gcsFiles, models.GCSFile{
			FileName:   file.Name(),
			Reader:     fileReader,
			BucketName: bucketName,
		})
	}

	return gcsFiles, nil
}

func (repo *GcsRepositoryFake) GetFile(ctx context.Context, bucketName, fileName string) (models.GCSFile, error) {
	cwd, _ := os.Getwd()
	fileNameElements := strings.Split(fileName, "/")
	path := filepath.Join(cwd, tempFilesDirectory, fileNameElements[len(fileNameElements)-1])
	file, err := os.Open(path)
	if err != nil {
		panic(err)
	}

	return models.GCSFile{
		FileName:   fileName,
		Reader:     file,
		BucketName: bucketName,
	}, nil
}

func (repo *GcsRepositoryFake) MoveFile(ctx context.Context, bucketName, source, destination string) error {
	return nil
}

func (repo *GcsRepositoryFake) OpenStream(ctx context.Context, bucketName, fileName string) io.WriteCloser {
	cwd, _ := os.Getwd()
	if _, err := os.Stat(tempFilesDirectory); os.IsNotExist(err) {
		err := os.Mkdir(tempFilesDirectory, os.ModePerm)
		if err != nil {
			panic(err)
		}
	}

	fileNameElements := strings.Split(fileName, "/")
	file, err := os.Create(filepath.Join(cwd, tempFilesDirectory, fileNameElements[len(fileNameElements)-1]))
	if err != nil {
		panic(err)
	}
	return file
}

func (repo *GcsRepositoryFake) UpdateFileMetadata(ctx context.Context, bucketName, fileName string, metadata map[string]string) error {
	return nil
}
