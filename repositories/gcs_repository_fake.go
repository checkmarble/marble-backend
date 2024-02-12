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
	sanitizedFileName := strings.ReplaceAll(fileName, "/", ":") // Workaround because slashes are not allowed in file names
	path := filepath.Join(cwd, tempFilesDirectory, sanitizedFileName)
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

	sanitizedFileName := strings.ReplaceAll(fileName, "/", ":") // Workaround because slashes are not allowed in file names
	file, err := os.Create(filepath.Join(cwd, tempFilesDirectory, sanitizedFileName))
	if err != nil {
		panic(err)
	}
	return file
}

func (repo *GcsRepositoryFake) UpdateFileMetadata(ctx context.Context, bucketName, fileName string, metadata map[string]string) error {
	return nil
}

func (repo *GcsRepositoryFake) DeleteFile(ctx context.Context, bucketName, fileName string) error {
	return nil
}

func (repo *GcsRepositoryFake) GenerateSignedUrl(ctx context.Context, bucketName, fileName string) (string, error) {
	// dummy file, url valid for 3 years from 2023/12/15
	return "https://storage.googleapis.com/data-ingestion-tokyo-country-381508/test.csv?Expires=1797266654&GoogleAccessId=admintest%40tokyo-country-381508.iam.gserviceaccount.com&Signature=YAVmUMWzR9sQBg9pZiDI%2FOnjRmun%2BT3Mkn84cGb%2FzYdd%2FGovpm6BNV928rAlFF33LnbmEr6JpdnW1SnA72dEOaWqOhRSWuw9pIPkxyZerD9NJyHXCmRSoSSwX7TDHKZZ0lIxz%2FxE8Wtu2Y7Q1Wn83tpigH1y8FNguSX8Zz4OjMKCSSbEXY5PsazNl12yj%2Bp8loqRwG9XIYXstLp0wKpdryz7WkqzORays7OuPs0uPoNFpTgEZtUhaoHTzRV%2FHEHnvEQ0FVFxNYnuTBPyeA%2FADlaSwDxRfGZbt65E4k73XgS1oMgdboPeCEopKAZ0Iikg7th1wdzrfetipvTucWpKOg%3D%3D", nil
}
