package agent_dto

import (
	"encoding/json"

	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
)

// This struct is used as a DTO, but instead of using struct tags directly they are set on an anonymous struct below in the MarshalJSON method.
// This is so we can set the RelatedObjects and Data to empty slices/maps if they are nil, and avoid return a null array/object.
type ClientObjectDetail struct {
	ObjectType     string
	Data           map[string]any
	RelatedObjects []RelatedObject
	Annotations    dto.GroupedEntityAnnotations
}

// ⚠️ this function is used recursively due to ClientObjectDetail.RelatedObjects. Handle with care.
func AdaptClientObjectDetailDto(c models.ClientObjectDetail) (ClientObjectDetail, error) {
	var relatedObjects []RelatedObject
	// This check is important to avoid infinite recursion
	if len(relatedObjects) > 0 {
		var err error
		relatedObjects, err = pure_utils.MapErr(c.RelatedObjects, AdaptRelatedObjectDto)
		if err != nil {
			return ClientObjectDetail{}, err
		}
	}

	out := ClientObjectDetail{
		ObjectType:     c.Metadata.ObjectType,
		Data:           c.Data,
		RelatedObjects: relatedObjects,
	}

	annotations, err := dto.AdaptGroupedEntityAnnotations(c.Annotations)
	if err != nil {
		return out, err
	}
	out.Annotations = annotations

	return out, nil
}

func (c ClientObjectDetail) MarshalJSON() ([]byte, error) {
	if c.RelatedObjects == nil {
		c.RelatedObjects = make([]RelatedObject, 0)
	}
	if c.Data == nil {
		c.Data = make(map[string]any)
	}
	return json.Marshal(struct {
		ObjectType     string                       `json:"object_type"`
		Data           map[string]any               `json:"data"`
		RelatedObjects []RelatedObject              `json:"related_objects"`
		Annotations    dto.GroupedEntityAnnotations `json:"annotations,omitzero"`
	}{
		ObjectType:     c.ObjectType,
		Data:           c.Data,
		RelatedObjects: c.RelatedObjects,
		Annotations:    c.Annotations,
	})
}

type RelatedObject struct {
	LinkName string             `json:"link_name"`
	Detail   ClientObjectDetail `json:"related_object_detail"` //nolint:tagliatelle
}

func AdaptRelatedObjectDto(o models.RelatedObject) (RelatedObject, error) {
	c, err := AdaptClientObjectDetailDto(o.Detail)
	return RelatedObject{
		LinkName: o.LinkName,
		Detail:   c,
	}, err
}

type PivotObject struct {
	PivotObjectId     string             `json:"pivot_object_id"`
	PivotValue        string             `json:"pivot_value"`
	PivotObjectName   string             `json:"pivot_object_name"`
	PivotFieldName    string             `json:"pivot_field_name"`
	PivotObjectData   ClientObjectDetail `json:"pivot_object_data"`
	NumberOfDecisions int                `json:"number_of_decisions"`
}

func AdaptPivotObjectDto(p models.PivotObject) (PivotObject, error) {
	pivotObjectData, err := AdaptClientObjectDetailDto(p.PivotObjectData)
	if err != nil {
		return PivotObject{}, err
	}
	return PivotObject{
		PivotObjectId:     p.PivotObjectId,
		PivotValue:        p.PivotValue,
		PivotObjectName:   p.PivotObjectName,
		PivotFieldName:    p.PivotFieldName,
		PivotObjectData:   pivotObjectData,
		NumberOfDecisions: p.NumberOfDecisions,
	}, nil
}
