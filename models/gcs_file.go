package models

import "io"

type GCSFile struct {
	FileName   string
	Reader     io.Reader
	BucketName string
}
