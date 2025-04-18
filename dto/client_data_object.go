package dto

import (
	"encoding/json"

	"github.com/checkmarble/marble-backend/models"
)

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
	PivotObjectId     string                    `json:"pivot_object_id"`
	PivotValue        string                    `json:"pivot_value"`
	PivotId           string                    `json:"pivot_id"`
	PivotType         string                    `json:"pivot_type"`
	PivotObjectName   string                    `json:"pivot_object_name"`
	PivotFieldName    string                    `json:"pivot_field_name"`
	IsIngested        bool                      `json:"is_ingested"`
	PivotObjectData   models.ClientObjectDetail `json:"pivot_object_data"`
	NumberOfDecisions int                       `json:"number_of_decisions"`
	Annotations       GroupedEntityAnnotations  `json:"annotations"`
}

func AdaptPivotObjectDto(p models.PivotObject) (PivotObject, error) {
	annotations, err := AdaptGroupedEntityAnnotations(p.Annotations)
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
		PivotObjectData:   p.PivotObjectData,
		NumberOfDecisions: p.NumberOfDecisions,
		Annotations:       annotations,
	}, nil
}

type GroupedEntityAnnotations struct {
	Comments []EntityAnnotationDto `json:"comments"`
	Tags     []EntityAnnotationDto `json:"tags"`
	Files    []EntityAnnotationDto `json:"files"`
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
