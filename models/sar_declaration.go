package models

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

// SarErrorCode is the stable, machine-readable code of a refused SAR generation
// operation. The analyst front maps it to its own handling.
type SarErrorCode string

const SarCodeVersionConflict SarErrorCode = "sar_version_conflict"

// SarError is a refusal of a SAR generation operation: a stable Code and a
// clean French Message the analyst reads as is. It wraps one of the generic
// sentinels (BadParameterError, ConflictError, ForbiddenError, NotFoundError),
// so errors.Is on the sentinel and the HTTP status mapping keep working, but
// its Error() text is the Message alone (no sentinel suffix, no wrapper).
type SarError struct {
	Code    SarErrorCode
	Message string
	kind    error
}

// NewSarError builds a refusal of the given kind (a generic sentinel).
func NewSarError(kind error, code SarErrorCode, message string) *SarError {
	return &SarError{Code: code, Message: message, kind: kind}
}

// NewSarErrorf is NewSarError with a formatted message.
func NewSarErrorf(kind error, code SarErrorCode, format string, args ...any) *SarError {
	return NewSarError(kind, code, fmt.Sprintf(format, args...))
}

func (e *SarError) Error() string { return e.Message }
func (e *SarError) Unwrap() error { return e.kind }

// ErrSarVersionConflict is returned when a declaration was modified since the
// version the caller based its update on.
var ErrSarVersionConflict = NewSarError(ConflictError, SarCodeVersionConflict,
	"la déclaration a été modifiée entre-temps")

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
