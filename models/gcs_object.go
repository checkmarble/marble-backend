package models

import "io"

type GCSObject struct {
	FileName string
	Reader   io.Reader
}
