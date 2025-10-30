package models

import (
	"encoding/json"

	"github.com/google/uuid"
)

type InsertScreeningMonitoringObject struct {
	ObjectType    string
	ConfigId      uuid.UUID
	ObjectId      *string
	ObjectPayload *json.RawMessage
}
