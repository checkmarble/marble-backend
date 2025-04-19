package dto

import (
	"encoding/json"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
)

// This struct is used as a DTO, but instead of using struct tags directly they are set on an anonymous struct below in the MarshalJSON method.
// This is so we can set the RelatedObjects and Data to empty slices/maps if they are nil, and avoid return a null array/object.
type ClientObjectDetail struct {
	Metadata       ClientObjectMetadata
	Data           map[string]any
	RelatedObjects []RelatedObject
	Annotations    GroupedEntityAnnotations
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
		Metadata: ClientObjectMetadata{
			ValidFrom:  c.Metadata.ValidFrom,
			ObjectType: c.Metadata.ObjectType,
		},
		Data:           c.Data,
		RelatedObjects: relatedObjects,
	}

	annotations, err := AdaptGroupedEntityAnnotations(c.Annotations)
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
		Metadata       ClientObjectMetadata     `json:"metadata,omitzero"`
		Data           map[string]any           `json:"data"`
		RelatedObjects []RelatedObject          `json:"related_objects"`
		Annotations    GroupedEntityAnnotations `json:"annotations,omitzero"`
	}{
		Metadata:       c.Metadata,
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

type ClientObjectMetadata struct {
	ValidFrom  time.Time `json:"valid_from"`
	ObjectType string    `json:"object_type"`
}

type ClientDataListResponse struct {
	Data       []models.ClientObjectDetail `json:"data"`
	Pagination ClientDataListPagination    `json:"pagination"`
}

func (c ClientDataListResponse) MarshalJSON() ([]byte, error) {
	if c.Data == nil {
		c.Data = make([]models.ClientObjectDetail, 0)
	}
	return json.Marshal(struct {
		Data       []models.ClientObjectDetail `json:"data"`
		Pagination ClientDataListPagination    `json:"pagination"`
	}{
		Data:       c.Data,
		Pagination: c.Pagination,
	})
}

type ClientDataListPagination struct {
	NextCursorId *string `json:"next_cursor_id"`
	HasNextPage  bool    `json:"has_next_page"`
}

type ClientDataListRequestBody struct {
	ExplorationOptions ExplorationOptions `json:"exploration_options"`
	CursorId           *string            `json:"cursor_id"`
	Limit              *int               `json:"limit" validate:"gt=0,lte=100"`
}

type ExplorationOptions struct {
	SourceTableName   string         `json:"source_table_name"`
	FilterFieldName   string         `json:"filter_field_name"`
	FilterFieldValue  StringOrNumber `json:"filter_field_value"`
	OrderingFieldName string         `json:"ordering_field_name"`
}

func AdaptClientDataListRequestBody(input ClientDataListRequestBody) models.ClientDataListRequestBody {
	m := models.ClientDataListRequestBody{
		ExplorationOptions: models.ExplorationOptions{
			SourceTableName: input.ExplorationOptions.SourceTableName,
			FilterFieldName: input.ExplorationOptions.FilterFieldName,
			FilterFieldValue: AdaptStringOrNumber(
				input.ExplorationOptions.FilterFieldValue),
			OrderingFieldName: input.ExplorationOptions.OrderingFieldName,
		},
		CursorId: input.CursorId,
	}

	if input.Limit != nil {
		m.Limit = *input.Limit
	} else {
		m.Limit = 100
	}

	return m
}

func AdaptClientDataListPaginationDto(input models.ClientDataListPagination) ClientDataListPagination {
	return ClientDataListPagination{
		NextCursorId: input.NextCursorId,
		HasNextPage:  input.HasNextPage,
	}
}

type PivotObject struct {
	PivotObjectId     string             `json:"pivot_object_id"`
	PivotValue        string             `json:"pivot_value"`
	PivotId           string             `json:"pivot_id"`
	PivotType         string             `json:"pivot_type"`
	PivotObjectName   string             `json:"pivot_object_name"`
	PivotFieldName    string             `json:"pivot_field_name"`
	IsIngested        bool               `json:"is_ingested"`
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
		PivotId:           p.PivotId,
		PivotType:         p.PivotType.String(),
		PivotObjectName:   p.PivotObjectName,
		PivotFieldName:    p.PivotFieldName,
		IsIngested:        p.IsIngested,
		PivotObjectData:   pivotObjectData,
		NumberOfDecisions: p.NumberOfDecisions,
	}, nil
}

type GroupedEntityAnnotations struct {
	Comments []EntityAnnotationDto `json:"comments,omitzero"`
	Tags     []EntityAnnotationDto `json:"tags,omitzero"`
	Files    []EntityAnnotationDto `json:"files,omitzero"`
}

func AdaptGroupedEntityAnnotations(a models.GroupedEntityAnnotations) (GroupedEntityAnnotations, error) {
	out := GroupedEntityAnnotations{
		Comments: make([]EntityAnnotationDto, len(a.Comments)),
		Tags:     make([]EntityAnnotationDto, len(a.Tags)),
		Files:    make([]EntityAnnotationDto, len(a.Files)),
	}

	for i, comment := range a.Comments {
		dto, err := AdaptEntityAnnotation(comment)
		if err != nil {
			return GroupedEntityAnnotations{}, err
		}
		out.Comments[i] = dto
	}
	for i, tag := range a.Tags {
		dto, err := AdaptEntityAnnotation(tag)
		if err != nil {
			return GroupedEntityAnnotations{}, err
		}
		out.Tags[i] = dto
	}
	for i, file := range a.Files {
		dto, err := AdaptEntityAnnotation(file)
		if err != nil {
			return GroupedEntityAnnotations{}, err
		}
		out.Files[i] = dto
	}

	return out, nil
}
