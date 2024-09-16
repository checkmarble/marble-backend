package models

import (
	"io"
)

type Blob struct {
	FileName   string
	ReadCloser io.ReadCloser
}
