package models

// The payload of entity annotations is a polymorphic schema depending on the type of the annotation.
// The EntityAnnotationPayload is a marker interface used to limit what types can be used there.
//
// The schema for annotation is as following:
//
// Comments
//
// {
// 	 "text": "Freeform text"
// }
//
// Tags
// The UUID in the `tag` attribute must be an existing tag from the `tags` table.
//
// {
//   "tag": "<uuid>"
// }
//
// File
// Here, the storage and output format is different from the input one.
// When uploading a file, a form-data POST request is used, so we do not encode the request as JSON but use `caption=&files[]=` instead.
//
// Implementation details are also not marshalled when transmitting data back to the client, so `bucket` and `files.key` are omited from the JSON output.
//
// {
// 	"caption": "Freeform caption",
// 	"bucket": "<URL to the blog storage bucket storing the files>",
// 	"files": [
// 	  {
// 	    "id": "<uuid>",
// 		"key": "<blog storage key of the file>",
// 		"filename": "<original file name of the uploaded file>"
// 	  }
// 	]
// }

type EntityAnnotationPayload interface {
	entityAnnotationPayload()
}

type EntityAnnotationCommentPayload struct {
	Text string `json:"text" binding:"required"`
}

func (EntityAnnotationCommentPayload) entityAnnotationPayload() {}

type EntityAnnotationFilePayload struct {
	Caption string                            `json:"caption"`
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
	TagId string `json:"tag_id" binding:"required"`
}

func (EntityAnnotationTagPayload) entityAnnotationPayload() {}
