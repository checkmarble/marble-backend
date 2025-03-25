package models

type EntityAnnotationPayload interface {
	entityAnnotationPayload()
}

type EntityAnnotationCommentPayload struct {
	Text string `json:"text" binding:"required"`
}

func (EntityAnnotationCommentPayload) entityAnnotationPayload() {}

type EntityAnnotationFilePayload struct {
	Caption string                            `json:"caption" binding:"required"`
	Bucket  string                            `json:"bucket"`
	Files   []EntityAnnotationFilePayloadFile `json:"files"`
}

type EntityAnnotationFilePayloadFile struct {
	Id       string `json:"id"`
	Key      string `json:"key"`
	Filename string `json:"filename"`
}

func (EntityAnnotationFilePayload) entityAnnotationPayload() {}

type EntityAnnotationTagPayload struct {
	Tag string `json:"tag" binding:"required"`
}

func (EntityAnnotationTagPayload) entityAnnotationPayload() {}
