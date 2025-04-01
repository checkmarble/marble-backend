package models

import (
	"encoding/json"
	"time"
)

type EntityAnnotationType int

const (
	EntityAnnotationUnknown EntityAnnotationType = iota
	EntityAnnotationComment
	EntityAnnotationFile
	EntityAnnotationTag
)

func EntityAnnotationFrom(kind string) EntityAnnotationType {
	switch kind {
	case "comment":
		return EntityAnnotationComment
	case "file":
		return EntityAnnotationFile
	case "tag":
		return EntityAnnotationTag
	default:
		return EntityAnnotationUnknown
	}
}

func (t EntityAnnotationType) String() string {
	switch t {
	case EntityAnnotationComment:
		return "comment"
	case EntityAnnotationFile:
		return "file"
	case EntityAnnotationTag:
		return "tag"
	default:
		return "unknown"
	}
}

type EntityAnnotation struct {
	Id             string
	OrgId          string
	ObjectType     string
	ObjectId       string
	CaseId         *string
	AnnotationType EntityAnnotationType
	Payload        json.RawMessage
	AnnotatedBy    *UserId
	CreatedAt      time.Time
	DeletedAt      *time.Time
}

type EntityAnnotationRequest struct {
	OrgId          string
	ObjectType     string
	ObjectId       string
	AnnotationType *EntityAnnotationType
}

type CaseEntityAnnotationRequest struct {
	OrgId          string
	CaseId         string
	AnnotationType *EntityAnnotationType
}

type EntityAnnotationRequestForObjects struct {
	OrgId          string
	ObjectType     string
	ObjectIds      []string
	AnnotationType *EntityAnnotationType
}

type CreateEntityAnnotationRequest struct {
	OrgId          string
	ObjectType     string
	ObjectId       string
	CaseId         *string
	AnnotationType EntityAnnotationType
	Payload        EntityAnnotationPayload
	AnnotatedBy    *UserId
}

type AnnotationByIdRequest struct {
	OrgId          string
	AnnotationId   string
	AnnotationType *EntityAnnotationType
}
