package models

import "time"

type PivotMetadata struct {
	Id             string
	CreatedAt      time.Time
	OrganizationId string

	BaseTableId string
	FieldId     *string
	PathLinkIds []string
}

func AdaptPivot(pivotMeta PivotMetadata, dataModel DataModel) Pivot {
	pivot := Pivot{
		Id:             pivotMeta.Id,
		CreatedAt:      pivotMeta.CreatedAt,
		OrganizationId: pivotMeta.OrganizationId,

		BaseTableId: pivotMeta.BaseTableId,
		FieldId:     pivotMeta.FieldId,
		PathLinkIds: pivotMeta.PathLinkIds,
	}

	baseTable := dataModel.AllTablesAsMap()[pivotMeta.BaseTableId]
	pivot.BaseTable = baseTable.Name

	if pivot.FieldId != nil {
		field := dataModel.AllFieldsAsMap()[*pivot.FieldId]
		pivot.Field = &field.Name
	}

	pivot.PathLinks = make([]string, 0, len(pivot.PathLinkIds))
	allLinks := dataModel.AllLinksAsMap()
	for _, linkId := range pivot.PathLinkIds {
		link := allLinks[linkId]
		pivot.PathLinks = append(pivot.PathLinks, link.Name)
	}

	return pivot
}

type Pivot struct {
	Id             string
	CreatedAt      time.Time
	OrganizationId string

	BaseTable   string
	BaseTableId string

	Field   *string
	FieldId *string

	PathLinks   []string
	PathLinkIds []string
}

type CreatePivotInput struct {
	BaseTableId    string
	OrganizationId string
	FieldId        *string
	PathLinkIds    []string
}
