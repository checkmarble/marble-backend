package models

import (
	"encoding/json"
	"time"
)

type PivotObject struct {
	PivotObjectId     string             `json:"pivot_object_id"`
	PivotValue        string             `json:"pivot_value"`
	PivotId           string             `json:"pivot_id"`
	PivotType         string             `json:"pivot_type"` // TODO: replace by enum
	PivotObjectName   string             `json:"pivot_object_name"`
	PivotFieldName    string             `json:"pivot_field_name"`
	IsIngested        bool               `json:"is_ingested"`
	PivotObjectData   ClientObjectDetail `json:"pivot_object_data"`
	NumberOfDecisions int                `json:"number_of_decisions"`
}

type ClientObjectDetail struct {
	Metadata       ClientObjectMetadata
	Data           map[string]any
	RelatedObjects []RelatedObject
}

func (c ClientObjectDetail) MarshalJSON() ([]byte, error) {
	if c.RelatedObjects == nil {
		c.RelatedObjects = make([]RelatedObject, 0)
	}
	if c.Data == nil {
		c.Data = make(map[string]any)
	}
	return json.Marshal(struct {
		Metadata       ClientObjectMetadata `json:"metadata,omitzero"`
		Data           map[string]any       `json:"data"`
		RelatedObjects []RelatedObject      `json:"related_objects"`
	}{
		Metadata:       c.Metadata,
		Data:           c.Data,
		RelatedObjects: c.RelatedObjects,
	})
}

type RelatedObject struct {
	LinkName string             `json:"link_name"`
	Detail   ClientObjectDetail `json:"related_object_detail"`
}

type ClientObjectMetadata struct {
	ValidFrom  time.Time `json:"valid_from"`
	ObjectType string    `json:"object_type"`
}
