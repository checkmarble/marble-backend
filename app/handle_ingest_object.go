package app

import (
	"fmt"
)

type IngestPayload struct {
	ObjectType string
	ObjectBody []byte
}

func (a *App) IngestObject(organizationID string, ingestPayload IngestPayload) (err error) {
	fmt.Println(ingestPayload)
	return a.repository.IngestObject(organizationID, ingestPayload)
}
