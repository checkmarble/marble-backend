package models

import (
	"fmt"
	"time"

	"github.com/checkmarble/marble-backend/pure_utils"
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

type Pivot struct {
	Id             string
	CreatedAt      time.Time
	OrganizationId string

	BaseTable    string
	BaseTableId  string
	PivotTable   string
	PivotTableId string

	Field   string
	FieldId string

	PathLinks   []string
	PathLinkIds []string
}

func (p PivotMetadata) Enrich(dataModel DataModel) Pivot {
	pivot := Pivot{
		Id:             p.Id,
		CreatedAt:      p.CreatedAt,
		OrganizationId: p.OrganizationId,

		BaseTableId: p.BaseTableId,
		PathLinkIds: p.PathLinkIds,
	}

	baseTable := dataModel.AllTablesAsMap()[p.BaseTableId]
	pivot.BaseTable = baseTable.Name

	if p.FieldId != nil {
		field := dataModel.AllFieldsAsMap()[*p.FieldId]
		pivot.Field = field.Name
		pivot.FieldId = field.ID
		// in this case, the pivot table is the base table
		pivot.PivotTable = baseTable.Name
		pivot.PivotTableId = baseTable.ID
	} else {
		field := FieldFromPath(dataModel, pivot.PathLinkIds)
		pivot.Field = field.Name
		pivot.FieldId = field.ID
		// in this case, the pivot table is the last table in the path
		pivot.PivotTable = dataModel.AllTablesAsMap()[field.TableId].Name
		pivot.PivotTableId = field.TableId
	}

	pivot.PathLinks = make([]string, 0, len(pivot.PathLinkIds))
	allLinks := dataModel.AllLinksAsMap()
	for _, linkId := range pivot.PathLinkIds {
		link := allLinks[linkId]
		pivot.PathLinks = append(pivot.PathLinks, link.Name)
	}

	return pivot
}

func ValidatePathPivot(dm DataModel, pathLinkIds []string, baseTableName string) error {
	linksMap := dm.AllLinksAsMap()
	// check that the first link is from the base table
	firstLink := linksMap[pathLinkIds[0]]
	if firstLink.ChildTableName != baseTableName {
		return errors.Wrap(
			BadParameterError,
			fmt.Sprintf(`first link's (%s) child table must be the base table "%s" (is "%s" instead)`,
				firstLink.Id, baseTableName, firstLink.ChildTableName,
			),
		)
	}

	// check that the links are chained consistently
	for i := 1; i < len(pathLinkIds); i++ {
		previousLink := linksMap[pathLinkIds[i-1]]
		currentLink := linksMap[pathLinkIds[i]]
		if previousLink.ParentTableName != currentLink.ChildTableName {
			return errors.Wrap(
				BadParameterError,
				fmt.Sprintf(`link %s (parent table "%s") is not a child of link %s (child table "%s")`,
					previousLink.Id, previousLink.ParentTableName, currentLink.Id, currentLink.ChildTableName,
				),
			)
		}
	}

	return nil
}

func FieldFromPath(dm DataModel, pathLinkIds []string) Field {
	// at this point the path is validated, so we assume that pathLinkIds is not empty and that the fieldId is found in the data model
	linksMap := dm.AllLinksAsMap()
	lastLink := linksMap[pathLinkIds[len(pathLinkIds)-1]]
	return dm.AllFieldsAsMap()[lastLink.ParentFieldId]
}

// Find the pivot definition, if there is one for this table
func FindPivot(pivotsMeta []PivotMetadata, table string, dm DataModel) *Pivot {
	pivots := pure_utils.Map(pivotsMeta, func(p PivotMetadata) Pivot {
		return p.Enrich(dm)
	})
	var pivot *Pivot
	for _, p := range pivots {
		if p.BaseTable == table {
			pivot = &p
			break
		}
	}

	return pivot
}

type CreatePivotInput struct {
	BaseTableId    string
	OrganizationId string
	FieldId        *string
	PathLinkIds    []string
}

// Pivot data, from decisions
type PivotDataWithCount struct {
	PivotId       string
	PivotValue    string
	NbOfDecisions int
}
