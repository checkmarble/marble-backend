package params

import "encoding/json"

type AttachClientDataAnnotationParams struct {
	Type string `json:"type" binding:"required"`

	// See description of the payload schema in models/entity_annotation_payload.go
	Payload json.RawMessage `json:"payload"`
}
