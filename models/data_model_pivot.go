package models

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
)

type PivotMetadata struct {
	Id             uuid.UUID
	CreatedAt      time.Time
	OrganizationId uuid.UUID

	BaseTableId string
	FieldId     *string
	PathLinkIds []string

	// Pivots are soft-deleted: decisions keep referencing their pivot_id after the
	// pivot is removed, so the definition must remain readable for historical data.
	DeletedAt *time.Time
}

type Pivot struct {
	Id             uuid.UUID
	CreatedAt      time.Time
	OrganizationId uuid.UUID

	BaseTable    string
	BaseTableId  string
	PivotTable   string
	PivotTableId string

	Field   string
	FieldId string

	PathLinks   []string
	PathLinkIds []string

	DeletedAt *time.Time
}

func (p PivotMetadata) Enrich(dataModel DataModel) Pivot {
	pivot := Pivot{
		Id:             p.Id,
		CreatedAt:      p.CreatedAt,
		OrganizationId: p.OrganizationId,

		BaseTableId: p.BaseTableId,
		PathLinkIds: p.PathLinkIds,
		DeletedAt:   p.DeletedAt,
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

// FindPivotsForTable returns every pivot definition whose base table is the given
// table. A table may have several pivots (polymorphic belongs_to: at most one
// applies per row); the per-row resolution of which one applies happens at
// evaluation time. The result is sorted deterministically (by creation time then
// id) so that "first non-null path wins" resolution is stable.
func FindPivotsForTable(pivotsMeta []PivotMetadata, table string, dm DataModel) []Pivot {
	pivots := make([]Pivot, 0, len(pivotsMeta))
	for _, p := range pivotsMeta {
		enriched := p.Enrich(dm)
		if enriched.BaseTable == table {
			pivots = append(pivots, enriched)
		}
	}

	slices.SortFunc(pivots, func(a, b Pivot) int {
		if c := a.CreatedAt.Compare(b.CreatedAt); c != 0 {
			return c
		}
		return strings.Compare(a.Id.String(), b.Id.String())
	})

	return pivots
}

type CreatePivotInput struct {
	BaseTableId    string
	OrganizationId uuid.UUID
	FieldId        *string
	PathLinkIds    []string
}

// Pivot data, from decisions
type PivotDataWithCount struct {
	PivotId       string
	PivotValue    string
	NbOfDecisions int
}
