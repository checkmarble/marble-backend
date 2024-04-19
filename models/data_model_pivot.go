package models

import "time"

type Pivot struct {
	Id string

	BaseTable   string
	BaseTableId string

	CreatedAt time.Time

	BaseField   *string
	BaseFieldId *string

	Links   []string
	LinkIds []string
}

type CreatePivotInput struct {
	BaseTableId string
	BaseFieldId *string
	LinkIds     []string
}
