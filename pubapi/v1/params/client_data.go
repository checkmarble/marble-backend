package params

import (
	"encoding/json"
	"mime/multipart"
)

type AttachClientDataAnnotationParams struct {
	Type string `json:"type" binding:"required"`

	// See description of the payload schema in models/entity_annotation_payload.go
	Payload json.RawMessage `json:"payload"`
}

type AttachClientDataFileAnnotationParams struct {
	Caption string                 `form:"caption" binding:"required"`
	Files   []multipart.FileHeader `form:"files[]" binding:"gte=1"`
}
