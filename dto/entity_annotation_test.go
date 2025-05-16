package dto

import (
	"testing"

	"github.com/checkmarble/marble-backend/models"
	"github.com/stretchr/testify/assert"
)

func TestDecodeEntityAnnotation(t *testing.T) {
	tts := []struct {
		kind    models.EntityAnnotationType
		payload []byte
		ok      bool
	}{
		{models.EntityAnnotationComment, []byte(`{"text":"comment text"}`), true},
		{models.EntityAnnotationTag, []byte(`{"tag_id": "tag_id"}`), true},

		{models.EntityAnnotationComment, []byte(`{}`), false},
		{models.EntityAnnotationTag, []byte(`{}`), false},

		// Files should always error out
		{models.EntityAnnotationFile, []byte(`{}`), false},
		{models.EntityAnnotationFile, []byte(`{"caption":"thecaption", "bucket":"bucket_name", "files":[{"id": "theid", "key": "thekey", "filename": "thefilename"}]}`), false},
	}

	for _, tt := range tts {
		genericPayload, err := DecodeEntityAnnotationPayload(tt.kind, tt.payload)

		if !tt.ok {
			assert.Error(t, err)
			continue
		}

		assert.NoError(t, err)

		switch tt.kind {
		case models.EntityAnnotationComment:
			payload, ok := genericPayload.(models.EntityAnnotationCommentPayload)

			assert.True(t, ok)
			assert.Equal(t, "comment text", payload.Text)

		case models.EntityAnnotationTag:
			payload, ok := genericPayload.(models.EntityAnnotationTagPayload)

			assert.True(t, ok)
			assert.Equal(t, "tag_id", payload.TagId)

		case models.EntityAnnotationFile:
			payload, ok := genericPayload.(models.EntityAnnotationFilePayload)

			assert.True(t, ok)
			assert.Equal(t, "thecaption", payload.Caption)
		}
	}
}

func TestAdaptEntityAnnotation(t *testing.T) {
	tts := []struct {
		kind    models.EntityAnnotationType
		payload []byte
	}{
		{models.EntityAnnotationComment, []byte(`{"text":"comment text"}`)},
		{models.EntityAnnotationTag, []byte(`{"tag": "tag_id"}`)},
		{models.EntityAnnotationFile, []byte(`{"caption":"thecaption", "bucket":"bucket_name", "files":[{"id": "theid", "key": "thekey", "filename": "thefilename"}]}`)},
	}

	for _, tt := range tts {
		genericPayload, err := AdaptEntityAnnotationPayload(models.EntityAnnotation{
			AnnotationType: tt.kind,
			Payload:        tt.payload,
		})

		assert.NoError(t, err)

		switch tt.kind {
		case models.EntityAnnotationComment:
			payload, ok := genericPayload.(EntityAnnotationCommentDto)

			assert.True(t, ok)
			assert.Equal(t, "comment text", payload.Text)

		case models.EntityAnnotationTag:
			payload, ok := genericPayload.(EntityAnnotationTagDto)

			assert.True(t, ok)
			assert.Equal(t, "tag_id", payload.Tag)

		case models.EntityAnnotationFile:
			payload, ok := genericPayload.(EntityAnnotationFileDto)

			assert.True(t, ok)
			assert.Equal(t, "thecaption", payload.Caption)
			assert.Len(t, payload.Files, 1)
			assert.Equal(t, "theid", payload.Files[0].Id)
			assert.Equal(t, "thefilename", payload.Files[0].Filename)
		}
	}
}
