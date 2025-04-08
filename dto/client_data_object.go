package dto

import "github.com/checkmarble/marble-backend/models"

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
}

func AdaptPivotObjectDto(p models.PivotObject) PivotObject {
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
	}
}
