package models

import (
	"time"

	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
)

// ErrSarVersionConflict is returned when a declaration was modified since the
// version the caller based its update on.
var ErrSarVersionConflict = errors.Wrap(ConflictError, "la déclaration a été modifiée entre-temps")

type SarDeclaration struct {
	Id             uuid.UUID
	OrganizationId uuid.UUID
	CaseId         uuid.UUID
	FormatKey      string
	ReportType     string
	Document       []byte
	Version        int
	Analysis       string
	ExtraInfo      string
	CreatedBy      *uuid.UUID
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type SarOrgFormatSettings struct {
	OrganizationId uuid.UUID
	FormatFamily   string
	Values         map[string]string
	UpdatedAt      time.Time
}

type SarDeclarationExport struct {
	Id                 uuid.UUID
	DeclarationId      uuid.UUID
	DeclarationVersion int
	Document           []byte
	Outstanding        []byte
	BucketName         string
	FileReference      string
	Sha256             string
	CreatedBy          *uuid.UUID
	CreatedAt          time.Time
}
