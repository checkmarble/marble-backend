package models

import (
	"cloud.google.com/go/storage"
)

type GCSFile struct {
	FileName   string
	Reader     *storage.Reader
	BucketName string
}
