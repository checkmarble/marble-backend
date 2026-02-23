package params

import (
	"encoding/json"
	"mime/multipart"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pubapi/types"
	"github.com/cockroachdb/errors"
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

// Define struct for annotation payloads
type commentAnnotationPayload struct {
	Text string `json:"text"`
}

type tagAnnotationPayload struct {
	TagId string `json:"tag_id"`
}

// riskTagAnnotationPayload intentionally excludes ContinuousScreeningId and
// OpenSanctionsEntityId, which are system-managed fields set by the continuous
// screening pipeline and must not be set by external clients.
type riskTagAnnotationPayload struct {
	Tag    models.RiskTag `json:"tag"`
	Reason string         `json:"reason"`
	Url    string         `json:"url"`
}

// DecodeAnnotationPayload parses and validates the raw JSON payload for the given annotation type,
// then converts it to the corresponding model type.
// The "file" type is not handled here as file annotations are created through multipart form data.
func DecodeAnnotationPayload(kind models.EntityAnnotationType, payload json.RawMessage) (models.EntityAnnotationPayload, error) {
	switch kind {
	case models.EntityAnnotationComment:
		var o commentAnnotationPayload
		if err := json.Unmarshal(payload, &o); err != nil {
			return nil, errors.WithDetail(types.ErrInvalidPayload, err.Error())
		}
		if o.Text == "" {
			return nil, errors.WithDetail(types.ErrInvalidPayload, "text is required")
		}
		return models.EntityAnnotationCommentPayload{Text: o.Text}, nil

	case models.EntityAnnotationTag:
		var o tagAnnotationPayload
		if err := json.Unmarshal(payload, &o); err != nil {
			return nil, errors.WithDetail(types.ErrInvalidPayload, err.Error())
		}
		return models.EntityAnnotationTagPayload{TagId: o.TagId}, nil

	case models.EntityAnnotationRiskTag:
		var o riskTagAnnotationPayload
		if err := json.Unmarshal(payload, &o); err != nil {
			return nil, errors.WithDetail(types.ErrInvalidPayload, err.Error())
		}
		return models.EntityAnnotationRiskTagPayload{
			Tag:    o.Tag,
			Reason: o.Reason,
			Url:    o.Url,
		}, nil

	default:
		return nil, errors.WithDetail(types.ErrInvalidPayload, "unsupported annotation type")
	}
}
