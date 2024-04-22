package models

import (
	"fmt"
	"time"

	"github.com/cockroachdb/errors"
)

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
		PathLinkIds: pivotMeta.PathLinkIds,
	}

	baseTable := dataModel.AllTablesAsMap()[pivotMeta.BaseTableId]
	pivot.BaseTable = baseTable.Name

	if pivotMeta.FieldId != nil {
		field := dataModel.AllFieldsAsMap()[*pivotMeta.FieldId]
		pivot.Field = field.Name
		pivot.FieldId = field.ID
	} else {
		field, _ := FieldFromPath(dataModel, pivot.PathLinkIds, pivot.BaseTable)
		pivot.Field = field.Name
		pivot.FieldId = field.ID
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

	Field   string
	FieldId string

	PathLinks   []string
	PathLinkIds []string
}

func FieldFromPath(dm DataModel, pathLinkIds []string, baseTableName string) (Field, error) {
	linksMap := dm.AllLinksAsMap()
	// check that the first link is from the base table
	firstLink := linksMap[pathLinkIds[0]]
	if string(firstLink.ChildTableName) != baseTableName {
		return Field{}, errors.Wrap(
			BadParameterError,
			fmt.Sprintf(`first link's (%s) child table must be the base table "%s" (is "%s" instead)`,
				firstLink.Id, baseTableName, firstLink.ChildTableName,
			),
		)
	}

	var fieldId string
	// check that the links are chained consistently
	for i := 1; i < len(pathLinkIds); i++ {
		previousLink := linksMap[pathLinkIds[i-1]]
		currentLink := linksMap[pathLinkIds[i]]
		if previousLink.ParentTableName != currentLink.ChildTableName {
			return Field{}, errors.Wrap(
				BadParameterError,
				fmt.Sprintf(`link %s (parent table "%s") is not a child of link %s (child table "%s")`,
					previousLink.Id, previousLink.ParentTableName, currentLink.Id, currentLink.ChildTableName,
				),
			)
		}

		fieldId = currentLink.ParentFieldId
	}

	return dm.AllFieldsAsMap()[fieldId], nil
}

type CreatePivotInput struct {
	BaseTableId    string
	OrganizationId string
	FieldId        *string
	PathLinkIds    []string
}
